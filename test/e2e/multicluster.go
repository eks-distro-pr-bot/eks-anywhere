//go:build e2e
// +build e2e

package e2e

import (
	"time"

	"github.com/aws/eks-anywhere/internal/pkg/api"
	"github.com/aws/eks-anywhere/pkg/api/v1alpha1"
	releasev1 "github.com/aws/eks-anywhere/release/api/v1alpha1"
	"github.com/aws/eks-anywhere/test/framework"
)

func runWorkloadClusterFlow(test *framework.MulticlusterE2ETest) {
	test.CreateManagementClusterWithConfig()
	licenseToken2 := framework.GetLicenseToken2()
	test.RunInWorkloadClusters(func(w *framework.WorkloadCluster) {
		w.GenerateClusterConfigWithLicenseToken(licenseToken2)
		w.CreateCluster()
		w.DeleteCluster()
	})
	time.Sleep(5 * time.Minute)
	test.DeleteManagementCluster()
}

func runWorkloadClusterExistingConfigFlow(test *framework.MulticlusterE2ETest) {
	test.CreateManagementCluster()
	test.RunInWorkloadClusters(func(w *framework.WorkloadCluster) {
		w.CreateCluster()
		w.DeleteCluster()
	})
	time.Sleep(5 * time.Minute)
	test.DeleteManagementCluster()
}

func runWorkloadClusterPrevVersionCreateFlow(test *framework.MulticlusterE2ETest, latestMinorRelease *releasev1.EksARelease) {
	test.CreateManagementClusterWithConfig()
	test.RunInWorkloadClusters(func(w *framework.WorkloadCluster) {
		w.GenerateClusterConfigForVersion(latestMinorRelease.Version, "", framework.ExecuteWithEksaRelease(latestMinorRelease))
		w.CreateCluster(framework.ExecuteWithEksaRelease(latestMinorRelease))
		w.DeleteCluster()
	})
	test.DeleteManagementCluster()
}

func runWorkloadClusterFlowWithGitOps(test *framework.MulticlusterE2ETest, clusterOpts ...framework.ClusterE2ETestOpt) {
	test.CreateManagementClusterWithConfig()
	licenseToken := framework.GetLicenseToken2()
	test.RunInWorkloadClusters(func(w *framework.WorkloadCluster) {
		w.GenerateClusterConfigWithLicenseToken(licenseToken)
		w.CreateCluster()
		w.UpgradeWithGitOps(clusterOpts...)
		time.Sleep(5 * time.Minute)
		w.DeleteCluster()
	})
	time.Sleep(5 * time.Minute)
	test.DeleteManagementCluster()
}

func runWorkloadClusterGitOpsAPIFlowForBareMetal(test *framework.MulticlusterE2ETest) {
	test.CreateTinkerbellManagementCluster()
	test.RunInWorkloadClusters(func(w *framework.WorkloadCluster) {
		w.WaitForAvailableHardware()
		test.PushWorkloadClusterToGit(w)
		w.WaitForKubeconfig()
		w.ValidateClusterState()
		test.DeleteWorkloadClusterFromGit(w)
		w.ValidateClusterDelete()
		w.ValidateHardwareDecommissioned()
	})
	test.DeleteManagementCluster()
}

func runWorkloadClusterGitOpsAPIUpgradeFlowForBareMetal(test *framework.MulticlusterE2ETest, filler ...api.ClusterConfigFiller) {
	test.CreateTinkerbellManagementCluster()
	test.RunInWorkloadClusters(func(w *framework.WorkloadCluster) {
		w.WaitForAvailableHardware()
		test.PushWorkloadClusterToGit(w)
		w.WaitForKubeconfig()
		w.ValidateClusterState()
		test.PushWorkloadClusterToGit(w, filler...)
		w.ValidateClusterState()
		test.DeleteWorkloadClusterFromGit(w)
		w.ValidateClusterDelete()
		w.ValidateHardwareDecommissioned()
	})
	test.DeleteManagementCluster()
}

func runTinkerbellWorkloadClusterFlow(test *framework.MulticlusterE2ETest) {
	test.ManagementCluster.GenerateClusterConfig()
	test.CreateTinkerbellManagementCluster()
	licenseToken2 := framework.GetLicenseToken2()
	test.RunInWorkloadClusters(func(w *framework.WorkloadCluster) {
		w.GenerateClusterConfigWithLicenseToken(licenseToken2)
		w.CreateCluster(framework.WithControlPlaneWaitTimeout("20m"))
		w.StopIfFailed()
		w.DeleteCluster()
		w.ValidateHardwareDecommissioned()
	})
	test.DeleteTinkerbellManagementCluster()
}

func runWorkloadClusterWithAPIFlowForBareMetal(test *framework.MulticlusterE2ETest) {
	test.CreateTinkerbellManagementCluster()
	test.RunInWorkloadClusters(func(w *framework.WorkloadCluster) {
		w.WaitForAvailableHardware()
		w.ApplyClusterManifest()
		w.WaitForKubeconfig()
		w.ValidateClusterState()
		w.DeleteClusterWithKubectl()
		w.ValidateClusterDelete()
		w.ValidateHardwareDecommissioned()
	})
	test.DeleteTinkerbellManagementCluster()
}

func runSimpleWorkloadUpgradeFlowForBareMetal(test *framework.MulticlusterE2ETest, updateVersion v1alpha1.KubernetesVersion, clusterOpts ...framework.ClusterE2ETestOpt) {
	test.ManagementCluster.GenerateClusterConfig()
	test.CreateTinkerbellManagementCluster()
	test.RunInWorkloadClusters(func(w *framework.WorkloadCluster) {
		w.GenerateClusterConfig()
		w.CreateCluster(framework.WithControlPlaneWaitTimeout("20m"))
		time.Sleep(2 * time.Minute)
		w.UpgradeCluster(clusterOpts)
		time.Sleep(2 * time.Minute)
		w.ValidateCluster(updateVersion)
		w.StopIfFailed()
		w.DeleteCluster()
		w.ValidateHardwareDecommissioned()
	})
	test.DeleteManagementCluster()
}

func runWorkloadClusterUpgradeFlowWithAPIForBareMetal(test *framework.MulticlusterE2ETest, filler ...api.ClusterConfigFiller) {
	test.CreateTinkerbellManagementCluster()
	test.RunInWorkloadClusters(func(w *framework.WorkloadCluster) {
		w.WaitForAvailableHardware()
		w.ApplyClusterManifest()
		w.WaitForKubeconfig()
		w.ValidateClusterState()
		w.UpdateClusterConfig(filler...)
		w.ApplyClusterManifest()
		w.ValidateClusterState()
		w.DeleteClusterWithKubectl()
		w.ValidateClusterDelete()
		w.ValidateHardwareDecommissioned()
	})
	test.ManagementCluster.StopIfFailed()
	test.DeleteManagementCluster()
}

func runInPlaceWorkloadUpgradeFlow(test *framework.MulticlusterE2ETest, clusterOpts ...framework.ClusterE2ETestOpt) {
	test.CreateManagementCluster()
	test.RunInWorkloadClusters(func(w *framework.WorkloadCluster) {
		w.CreateCluster()
		w.UpgradeClusterWithNewConfig(clusterOpts)
		w.ValidateClusterState()
		w.StopIfFailed()
		w.DeleteCluster()
	})
	test.DeleteManagementCluster()
}
