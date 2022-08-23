package flux

import (
	"context"
	"fmt"
	"path"

	"github.com/aws/eks-anywhere/pkg/api/v1alpha1"
	"github.com/aws/eks-anywhere/pkg/cluster"
	"github.com/aws/eks-anywhere/pkg/config"
	"github.com/aws/eks-anywhere/pkg/filewriter"
	"github.com/aws/eks-anywhere/pkg/git"
	gitFactory "github.com/aws/eks-anywhere/pkg/git/factory"
	"github.com/aws/eks-anywhere/pkg/logger"
	"github.com/aws/eks-anywhere/pkg/providers"
	"github.com/aws/eks-anywhere/pkg/types"
	"github.com/aws/eks-anywhere/pkg/validations"
)

const (
	defaultRemote = "origin"

	initialClusterconfigCommitMessage = "Initial commit of cluster configuration; generated by EKS-A CLI"
	updateClusterconfigCommitMessage  = "Update commit of cluster configuration; generated by EKS-A CLI"
	deleteClusterconfigCommitMessage  = "Delete commit of cluster configuration; generated by EKS-A CLI"
)

type GitOpsFluxClient interface {
	BootstrapGithub(ctx context.Context, cluster *types.Cluster, fluxConfig *v1alpha1.FluxConfig) error
	BootstrapGit(ctx context.Context, cluster *types.Cluster, fluxConfig *v1alpha1.FluxConfig, cliConfig *config.CliConfig) error
	Uninstall(ctx context.Context, cluster *types.Cluster, fluxConfig *v1alpha1.FluxConfig) error
	GetCluster(ctx context.Context, cluster *types.Cluster, clusterSpec *cluster.Spec) (eksaCluster *v1alpha1.Cluster, err error)
	SuspendKustomization(ctx context.Context, cluster *types.Cluster, fluxConfig *v1alpha1.FluxConfig) error
	ResumeKustomization(ctx context.Context, cluster *types.Cluster, fluxConfig *v1alpha1.FluxConfig) error
	DisableResourceReconcile(ctx context.Context, cluster *types.Cluster, resourceType, objectName, namespace string) error
	EnableResourceReconcile(ctx context.Context, cluster *types.Cluster, resourceType, objectName, namespace string) error
	Reconcile(ctx context.Context, cluster *types.Cluster, fluxConfig *v1alpha1.FluxConfig) error
	ForceReconcile(ctx context.Context, cluster *types.Cluster, namespace string) error
	DeleteSystemSecret(ctx context.Context, cluster *types.Cluster, namespace string) error
}

type GitClient interface {
	GetRepo(ctx context.Context) (repo *git.Repository, err error)
	CreateRepo(ctx context.Context, opts git.CreateRepoOpts) error
	Clone(ctx context.Context) error
	Push(ctx context.Context) error
	Pull(ctx context.Context, branch string) error
	PathExists(ctx context.Context, owner, repo, branch, path string) (exists bool, err error)
	Add(filename string) error
	Remove(filename string) error
	Commit(message string) error
	Branch(name string) error
	Init() error
}

type Flux struct {
	fluxClient GitOpsFluxClient
	gitClient  GitClient
	writer     filewriter.FileWriter
	cliConfig  *config.CliConfig
}

func NewFlux(fluxClient FluxClient, kubeClient KubeClient, gitTools *gitFactory.GitTools, cliConfig *config.CliConfig) *Flux {
	var w filewriter.FileWriter
	if gitTools != nil {
		w = gitTools.Writer
	}

	return &Flux{
		fluxClient: newFluxClient(fluxClient, kubeClient),
		gitClient:  newGitClient(gitTools),
		writer:     w,
		cliConfig:  cliConfig,
	}
}

func NewFluxFromGitOpsFluxClient(fluxClient GitOpsFluxClient, gitClient GitClient, writer filewriter.FileWriter, cliConfig *config.CliConfig) *Flux {
	return &Flux{
		fluxClient: fluxClient,
		gitClient:  gitClient,
		writer:     writer,
		cliConfig:  cliConfig,
	}
}

func (f *Flux) InstallGitOps(ctx context.Context, cluster *types.Cluster, clusterSpec *cluster.Spec, datacenterConfig providers.DatacenterConfig, machineConfigs []providers.MachineConfig) error {
	if f.shouldSkipFlux() {
		logger.Info("GitOps field not specified, bootstrap flux skipped")
		return nil
	}

	fc := newFluxForCluster(f, clusterSpec, datacenterConfig, machineConfigs)

	if err := fc.setupRepository(ctx); err != nil {
		return err
	}

	if err := fc.commitFluxAndClusterConfigToGit(ctx); err != nil {
		return err
	}

	if err := f.Bootstrap(ctx, cluster, clusterSpec); err != nil {
		return err
	}

	logger.V(4).Info("pulling from remote after Flux Bootstrap to ensure configuration files in local git repository are in sync",
		"remote", defaultRemote, "branch", fc.branch())

	if err := f.gitClient.Pull(ctx, fc.branch()); err != nil {
		logger.Error(err, "error when pulling from remote repository after Flux Bootstrap; ensure local repository is up-to-date with remote (git pull)",
			"remote", defaultRemote, "branch", fc.branch(), "error", err)
	}
	return nil
}

func (f *Flux) Bootstrap(ctx context.Context, cluster *types.Cluster, clusterSpec *cluster.Spec) error {
	if err := f.BootstrapGithub(ctx, cluster, clusterSpec); err != nil {
		_ = f.Uninstall(ctx, cluster, clusterSpec)
		return fmt.Errorf("installing GitHub gitops: %v", err)
	}

	if err := f.BootstrapGit(ctx, cluster, clusterSpec); err != nil {
		_ = f.Uninstall(ctx, cluster, clusterSpec)
		return fmt.Errorf("installing generic git gitops: %v", err)
	}

	return nil
}

func (f *Flux) BootstrapGithub(ctx context.Context, cluster *types.Cluster, clusterSpec *cluster.Spec) error {
	if cluster.ExistingManagement || clusterSpec.FluxConfig.Spec.Github == nil {
		return nil
	}

	return f.fluxClient.BootstrapGithub(ctx, cluster, clusterSpec.FluxConfig)
}

func (f *Flux) BootstrapGit(ctx context.Context, cluster *types.Cluster, clusterSpec *cluster.Spec) error {
	if cluster.ExistingManagement || clusterSpec.FluxConfig.Spec.Git == nil {
		return nil
	}

	return f.fluxClient.BootstrapGit(ctx, cluster, clusterSpec.FluxConfig, f.cliConfig)
}

func (f *Flux) Uninstall(ctx context.Context, cluster *types.Cluster, clusterSpec *cluster.Spec) error {
	if err := f.fluxClient.Uninstall(ctx, cluster, clusterSpec.FluxConfig); err != nil {
		logger.Info("Could not uninstall flux components", "error", err)
		return err
	}
	return nil
}

func (f *Flux) PauseClusterResourcesReconcile(ctx context.Context, cluster *types.Cluster, clusterSpec *cluster.Spec, provider providers.Provider) error {
	if f.shouldSkipFlux() {
		logger.V(4).Info("GitOps field not specified, pause cluster resources reconcile skipped")
		return nil
	}

	logger.V(3).Info("Pause Flux EKS-A resources reconcile")

	if err := f.fluxClient.DisableResourceReconcile(ctx, cluster, clusterSpec.Cluster.ResourceType(), clusterSpec.Cluster.Name, clusterSpec.Cluster.Namespace); err != nil {
		return fmt.Errorf("disable resource %s %s from Flux reconcile: %v", clusterSpec.Cluster.ResourceType(), clusterSpec.Cluster.Name, err)
	}

	if err := f.fluxClient.DisableResourceReconcile(ctx, cluster, provider.DatacenterResourceType(), clusterSpec.Cluster.Spec.DatacenterRef.Name, clusterSpec.Cluster.Namespace); err != nil {
		return fmt.Errorf("disable resource %s %s from Flux reconcile: %v", provider.DatacenterResourceType(), clusterSpec.Cluster.Spec.DatacenterRef.Name, err)
	}

	if provider.MachineResourceType() != "" {
		for _, machineConfigRef := range clusterSpec.Cluster.MachineConfigRefs() {
			if err := f.fluxClient.DisableResourceReconcile(ctx, cluster, provider.MachineResourceType(), machineConfigRef.Name, clusterSpec.Cluster.Namespace); err != nil {
				return fmt.Errorf("disable resource %s %s from Flux reconcile: %v", provider.MachineResourceType(), machineConfigRef.Name, err)
			}
		}
	}

	return nil
}

func (f *Flux) ResumeClusterResourcesReconcile(ctx context.Context, cluster *types.Cluster, clusterSpec *cluster.Spec, provider providers.Provider) error {
	if f.shouldSkipFlux() {
		logger.V(4).Info("GitOps field not specified, resume cluster resources reconcile skipped")
		return nil
	}

	logger.V(3).Info("Resume Flux EKS-A resources reconcile")

	if err := f.fluxClient.EnableResourceReconcile(ctx, cluster, clusterSpec.Cluster.ResourceType(), clusterSpec.Cluster.Name, clusterSpec.Cluster.Namespace); err != nil {
		return fmt.Errorf("enable resource %s %s from Flux reconcile: %v", clusterSpec.Cluster.ResourceType(), clusterSpec.Cluster.Name, err)
	}

	if err := f.fluxClient.EnableResourceReconcile(ctx, cluster, provider.DatacenterResourceType(), clusterSpec.Cluster.Spec.DatacenterRef.Name, clusterSpec.Cluster.Namespace); err != nil {
		return fmt.Errorf("enable resource %s %s from Flux reconcile: %v", provider.DatacenterResourceType(), clusterSpec.Cluster.Spec.DatacenterRef.Name, err)
	}

	if provider.MachineResourceType() != "" {
		for _, machineConfigRef := range clusterSpec.Cluster.MachineConfigRefs() {
			if err := f.fluxClient.EnableResourceReconcile(ctx, cluster, provider.MachineResourceType(), machineConfigRef.Name, clusterSpec.Cluster.Namespace); err != nil {
				return fmt.Errorf("enable resource %s %s from Flux reconcile: %v", provider.MachineResourceType(), machineConfigRef.Name, err)
			}
		}
	}

	return nil
}

func (f *Flux) PauseGitOpsKustomization(ctx context.Context, cluster *types.Cluster, clusterSpec *cluster.Spec) error {
	if f.shouldSkipFlux() {
		logger.Info("GitOps field not specified, pause flux kustomization skipped")
		return nil
	}

	c, err := f.fluxClient.GetCluster(ctx, cluster, clusterSpec)
	if err != nil {
		return err
	}
	if c.Spec.GitOpsRef == nil {
		logger.Info("GitOps not enabled in the existing cluster, pause flux kustomization skipped")
		return nil
	}

	logger.V(3).Info("Pause reconciliation of all Kustomization", "namespace", clusterSpec.FluxConfig.Spec.SystemNamespace)

	return f.fluxClient.SuspendKustomization(ctx, cluster, clusterSpec.FluxConfig)
}

func (f *Flux) ResumeGitOpsKustomization(ctx context.Context, cluster *types.Cluster, clusterSpec *cluster.Spec) error {
	if f.shouldSkipFlux() {
		logger.Info("GitOps field not specified, resume flux kustomization skipped")
		return nil
	}

	logger.V(3).Info("resume reconciliation of all Kustomization", "namespace", clusterSpec.FluxConfig.Spec.SystemNamespace)
	return f.fluxClient.ResumeKustomization(ctx, cluster, clusterSpec.FluxConfig)
}

func (f *Flux) ForceReconcileGitRepo(ctx context.Context, cluster *types.Cluster, clusterSpec *cluster.Spec) error {
	if f.shouldSkipFlux() {
		logger.Info("GitOps not configured, force reconcile flux git repo skipped")
		return nil
	}

	return f.fluxClient.ForceReconcile(ctx, cluster, clusterSpec.FluxConfig.Spec.SystemNamespace)
}

func (f *Flux) UpdateGitEksaSpec(ctx context.Context, clusterSpec *cluster.Spec, datacenterConfig providers.DatacenterConfig, machineConfigs []providers.MachineConfig) error {
	if f.shouldSkipFlux() {
		logger.Info("GitOps field not specified, update git repo skipped")
		return nil
	}

	fc := newFluxForCluster(f, clusterSpec, datacenterConfig, machineConfigs)

	if err := fc.syncGitRepo(ctx); err != nil {
		return err
	}

	g := NewFileGenerator()
	if err := g.Init(f.writer, fc.eksaSystemDir(), fc.fluxSystemDir()); err != nil {
		return err
	}

	if err := g.WriteEksaFiles(clusterSpec, datacenterConfig, machineConfigs); err != nil {
		return err
	}

	path := fc.eksaSystemDir()
	if err := f.gitClient.Add(path); err != nil {
		return fmt.Errorf("adding %s to git: %v", path, err)
	}

	if err := f.pushToRemoteRepo(ctx, path, updateClusterconfigCommitMessage); err != nil {
		return err
	}
	logger.V(3).Info("Finished pushing updated cluster config file to git", "repository", fc.repository())
	return nil
}

func (f *Flux) Validations(ctx context.Context, clusterSpec *cluster.Spec) []validations.Validation {
	if f.shouldSkipFlux() {
		return nil
	}

	fc := newFluxForCluster(f, clusterSpec, nil, nil)

	return []validations.Validation{
		func() *validations.ValidationResult {
			return &validations.ValidationResult{
				Name:        "Flux path",
				Remediation: "Please provide a different path or different cluster name",
				Err:         fc.validateRemoteConfigPathDoesNotExist(ctx),
			}
		},
	}
}

func (f *Flux) CleanupGitRepo(ctx context.Context, clusterSpec *cluster.Spec) error {
	if f.shouldSkipFlux() {
		logger.Info("GitOps field not specified, clean up git repo skipped")
		return nil
	}

	fc := newFluxForCluster(f, clusterSpec, nil, nil)

	if err := fc.syncGitRepo(ctx); err != nil {
		return err
	}

	var p string
	if clusterSpec.Cluster.IsManaged() {
		p = fc.eksaSystemDir()
	} else {
		p = fc.path()
	}

	if !validations.FileExists(path.Join(f.writer.Dir(), p)) {
		logger.V(3).Info("cluster dir does not exist in git, skip clean up")
		return nil
	}

	if err := f.gitClient.Remove(p); err != nil {
		return fmt.Errorf("removing %s in git: %v", p, err)
	}

	if err := f.pushToRemoteRepo(ctx, p, deleteClusterconfigCommitMessage); err != nil {
		return err
	}

	logger.V(3).Info("Finished cleaning up cluster files in git",
		"repository", fc.repository())
	return nil
}

func (f *Flux) pushToRemoteRepo(ctx context.Context, path, msg string) error {
	if err := f.gitClient.Commit(msg); err != nil {
		return fmt.Errorf("committing %s to git: %v", path, err)
	}

	if err := f.gitClient.Push(ctx); err != nil {
		return fmt.Errorf("pushing %s to git: %v", path, err)
	}
	return nil
}

func (f *Flux) shouldSkipFlux() bool {
	return f.writer == nil
}
