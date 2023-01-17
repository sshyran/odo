package integration

import (
	"fmt"
	"path"
	"path/filepath"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/redhat-developer/odo/pkg/labels"
	"github.com/redhat-developer/odo/pkg/util"
	"github.com/redhat-developer/odo/tests/helper"
)

var _ = Describe("odo delete command tests", func() {
	var commonVar helper.CommonVar
	var cmpName, deploymentName, serviceName string
	var getDeployArgs, getSVCArgs []string

	// This is run before every Spec (It)
	var _ = BeforeEach(func() {
		commonVar = helper.CommonBeforeEach()
		cmpName = helper.RandString(6)
		helper.Chdir(commonVar.Context)
		getDeployArgs = []string{"get", "deployment", "-n", commonVar.Project}
		getSVCArgs = []string{"get", "svc", "-n", commonVar.Project}
	})

	// This is run after every Spec (It)
	var _ = AfterEach(func() {
		helper.CommonAfterEach(commonVar)
	})

	When("running odo delete from a non-component directory", func() {
		var files []string
		BeforeEach(func() {
			files = helper.ListFilesInDir(commonVar.Context)
			Expect(files).ToNot(ContainElement(".odo"))
		})

		check := func(opts ...string) {
			args := []string{"delete", "component", "-f"}
			args = append(args, opts...)
			errOut := helper.Cmd("odo", args...).ShouldFail().Err()
			helper.MatchAllInOutput(errOut, []string{"The current directory does not represent an odo component"})
		}

		When("the directory is empty", func() {
			BeforeEach(func() {
				Expect(len(files)).To(BeZero())
			})
			It("should fail", func() {
				By("not using --files", func() {
					check()
				})
				By("using --files", func() {
					check("--files")
				})
			})
		})
		When("the directory is not empty", func() {
			BeforeEach(func() {
				helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), commonVar.Context)
			})
			It("should fail", func() {
				By("not using --files", func() {
					check()
				})
				By("using --files", func() {
					check("--files")
				})
			})
		})
	})

	It("should fail when using both --files and --name", func() {
		errOut := helper.Cmd("odo", "delete", "component", "--name", "my-comp", "--files", "-f").ShouldFail().Err()
		helper.MatchAllInOutput(errOut, []string{"'--files' cannot be used with '--name'; '--files' must be used from a directory containing a Devfile"})
	})

	for _, withDotOdoDir := range []bool{true, false} {
		withDotOdoDir := withDotOdoDir
		When(fmt.Sprintf("using --files in a directory where Devfile was not generated by odo: withDotOdoDir=%v", withDotOdoDir), func() {
			var out string
			var fileList []string

			BeforeEach(func() {
				helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), commonVar.Context)
				helper.CopyExampleDevFile(
					filepath.Join("source", "devfiles", "nodejs", "devfile.yaml"),
					path.Join(commonVar.Context, "devfile.yaml"))
				if withDotOdoDir {
					helper.MakeDir(filepath.Join(commonVar.Context, util.DotOdoDirectory))
				}
				out = helper.Cmd("odo", "delete", "component", "--files", "-f").ShouldPass().Out()
				fileList = helper.ListFilesInDir(commonVar.Context)
			})

			It("should delete the relevant files", func() {
				By("not listing and deleting the devfile.yaml", func() {
					Expect(out).ShouldNot(ContainSubstring(filepath.Join(commonVar.Context, "devfile.yaml")))
					Expect(fileList).Should(ContainElement("devfile.yaml"))
				})

				if withDotOdoDir {
					By("listing and deleting the .odo directory", func() {
						Expect(out).Should(ContainSubstring(filepath.Join(commonVar.Context, util.DotOdoDirectory)))
						Expect(fileList).ShouldNot(ContainElement(util.DotOdoDirectory))
					})
				}
			})
		})
	}

	for _, ctx := range []struct {
		title       string
		devfileName string
		setupFunc   func()
	}{
		{
			title:       "a component is bootstrapped",
			devfileName: "devfile-deploy-with-multiple-resources.yaml",
		},
		{
			title:       "a component is bootstrapped using a devfile.yaml with URI-referenced Kubernetes components",
			devfileName: "devfile-deploy-with-multiple-resources-and-k8s-uri.yaml",
			setupFunc: func() {
				helper.CopyExample(
					filepath.Join("source", "devfiles", "nodejs", "kubernetes", "devfile-deploy-with-multiple-resources-and-k8s-uri"),
					filepath.Join(commonVar.Context, "kubernetes", "devfile-deploy-with-multiple-resources-and-k8s-uri"))
			},
		},
	} {
		// this is a workaround to ensure that the for loop works with `It` blocks
		ctx := ctx
		When(ctx.title, func() {
			BeforeEach(func() {
				deploymentName = "my-component"
				serviceName = "my-cs"
				helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), commonVar.Context)
				helper.Cmd("odo", "init", "--name", cmpName, "--devfile-path",
					helper.GetExamplePath("source", "devfiles", "nodejs", ctx.devfileName)).ShouldPass()
				// Note:	component will be automatically bootstrapped when `odo dev` or `odo deploy` is run
				if ctx.setupFunc != nil {
					ctx.setupFunc()
				}
			})
			When("the components are not deployed", func() {
				It("should output that there are no resources to be deleted", func() {
					stdOut := helper.Cmd("odo", "delete", "component", "-f").ShouldPass().Out()
					Expect(stdOut).To(ContainSubstring("No resource found for component %q in namespace %q", cmpName, commonVar.Project))
				})

				for _, label := range []string{
					helper.LabelNoCluster, helper.LabelUnauth,
				} {
					label := label
					It("should work without cluster", Label(label), func() {
						helper.Cmd("odo", "delete", "component").ShouldPass()
					})
				}

				It("should delete the respective files with --files", func() {
					stdOut := helper.Cmd("odo", "delete", "component", "--files", "-f").ShouldPass().Out()
					By("not finding resources in namepace", func() {
						Expect(stdOut).To(ContainSubstring("No resource found for component %q in namespace %q", cmpName, commonVar.Project))
					})
					By("listing files that were created by odo and that need to be deleted", func() {
						Expect(stdOut).To(ContainSubstring("This will delete the following files and directories"))
						//odo init above create the devfile.yaml, and so created the .odo/generated file as well
						Expect(stdOut).To(ContainSubstring(filepath.Join(commonVar.Context, "devfile.yaml")))
						Expect(stdOut).To(ContainSubstring(filepath.Join(commonVar.Context, util.DotOdoDirectory)))
					})
					By("deleting the expected files", func() {
						filesInDir := helper.ListFilesInDir(commonVar.Context)
						Expect(filesInDir).ShouldNot(ContainElement("devfile.yaml"))
						Expect(filesInDir).ShouldNot(ContainElement(util.DotOdoDirectory))
					})
				})
			})

			for _, podman := range []bool{true, false} {
				podman := podman
				When("the component is deployed in DEV mode and dev mode stopped", helper.LabelPodmanIf(podman, func() {
					var devSession helper.DevSession
					BeforeEach(func() {
						if podman {
							helper.EnableExperimentalMode()
						}
						var err error
						devSession, _, _, _, err = helper.StartDevMode(helper.DevSessionOpts{
							RunOnPodman: podman,
						})
						Expect(err).ToNot(HaveOccurred())

						devSession.Kill()
						devSession.WaitEnd()

						component := helper.NewComponent(cmpName, "app", labels.ComponentDevMode, commonVar.Project, commonVar.CliRunner)
						component.ExpectIsDeployed()
					})

					AfterEach(func() {
						if podman {
							helper.ResetExperimentalMode()
						}
					})

					When("the component is deleted using its name (and namespace) from another directory", func() {
						var out string
						BeforeEach(func() {
							otherDir := filepath.Join(commonVar.Context, "tmp")
							helper.MakeDir(otherDir)
							helper.Chdir(otherDir)
							args := []string{"delete", "component", "--name", cmpName, "-f"}
							if !podman {
								args = append(args, "--namespace", commonVar.Project)
							}
							out = helper.Cmd("odo", args...).ShouldPass().Out()
						})

						It("should have deleted the component", func() {
							By("listing the resource to delete", func() {
								if podman {
									Expect(out).To(ContainSubstring("- " + cmpName))
								} else {
									Expect(out).To(ContainSubstring("Deployment: " + cmpName))
								}
							})
							By("deleting the deployment", func() {
								component := helper.NewComponent(cmpName, "app", labels.ComponentDevMode, commonVar.Project, commonVar.CliRunner)
								component.ExpectIsNotDeployed()
							})
						})

						if !podman {
							When("odo delete command is run again with nothing deployed on the cluster", func() {
								var stdOut string
								BeforeEach(func() {
									// wait until the resources are deleted from the first delete
									Eventually(string(commonVar.CliRunner.Run(getDeployArgs...).Out.Contents()), 60, 3).ShouldNot(ContainSubstring(deploymentName))
									Eventually(string(commonVar.CliRunner.Run(getSVCArgs...).Out.Contents()), 60, 3).ShouldNot(ContainSubstring(serviceName))
								})
								It("should output that there are no resources to be deleted", func() {
									Eventually(func() string {
										stdOut = helper.Cmd("odo", "delete", "component", "--name", cmpName, "--namespace", commonVar.Project, "-f").ShouldPass().Out()
										return stdOut
									}, 60, 3).Should(ContainSubstring("No resource found for component %q in namespace %q", cmpName, commonVar.Project))
								})
							})
						}
					})

					Context("the component is deleted while having access to the devfile.yaml", func() {
						When("the component is deleted without --files", func() {
							var stdOut string
							BeforeEach(func() {
								stdOut = helper.Cmd("odo", "delete", "component", "-f").ShouldPass().Out()
							})

							It("should have deleted the component", func() {
								By("listing the resources to delete", func() {
									Expect(stdOut).To(ContainSubstring(cmpName))
								})
								By("deleting the deployment", func() {
									component := helper.NewComponent(cmpName, "app", labels.ComponentDevMode, commonVar.Project, commonVar.CliRunner)
									component.ExpectIsNotDeployed()
								})
								By("ensuring that devfile.yaml and .odo still exists", func() {
									files := helper.ListFilesInDir(commonVar.Context)
									Expect(files).To(ContainElement(util.DotOdoDirectory))
									Expect(files).To(ContainElement("devfile.yaml"))
								})
							})

							if !podman {
								When("odo delete command is run again with nothing deployed on the cluster", func() {
									var stdOut string
									BeforeEach(func() {
										// wait until the resources are deleted from the first delete
										Eventually(string(commonVar.CliRunner.Run(getDeployArgs...).Out.Contents()), 60, 3).ShouldNot(ContainSubstring(deploymentName))
										Eventually(string(commonVar.CliRunner.Run(getSVCArgs...).Out.Contents()), 60, 3).ShouldNot(ContainSubstring(serviceName))
										stdOut = helper.Cmd("odo", "delete", "component", "-f").ShouldPass().Out()
									})
									It("should output that there are no resources to be deleted", func() {
										Expect(stdOut).To(ContainSubstring("No resource found for component %q in namespace %q", cmpName, commonVar.Project))
									})
								})
							}
						})

						When("the component is deleted with --files", func() {
							var stdOut string
							BeforeEach(func() {
								stdOut = helper.Cmd("odo", "delete", "component", "--files", "-f").ShouldPass().Out()
							})

							It("should have deleted the component", func() {
								By("listing the resources to delete", func() {
									Expect(stdOut).To(ContainSubstring(cmpName))
								})
								By("deleting the deployment", func() {
									component := helper.NewComponent(cmpName, "app", labels.ComponentDevMode, commonVar.Project, commonVar.CliRunner)
									component.ExpectIsNotDeployed()
								})

								deletableFileNames := []string{util.DotOdoDirectory, "devfile.yaml"}
								var deletableFilesPaths []string
								By("listing the files to delete", func() {
									for _, f := range deletableFileNames {
										deletableFilesPaths = append(deletableFilesPaths, filepath.Join(commonVar.Context, f))
									}
									helper.MatchAllInOutput(stdOut, deletableFilesPaths)
								})
								By("ensuring that appropriate files have been removed", func() {
									files := helper.ListFilesInDir(commonVar.Context)
									for _, f := range deletableFileNames {
										Expect(files).ShouldNot(ContainElement(f))
									}
								})
							})
						})
					})
				}))
			}

			When("the component is deployed in DEPLOY mode", func() {
				BeforeEach(func() {
					helper.Cmd("odo", "deploy").AddEnv("PODMAN_CMD=echo").ShouldPass()
					Expect(commonVar.CliRunner.Run(getDeployArgs...).Out.Contents()).To(ContainSubstring(deploymentName))
					Expect(commonVar.CliRunner.Run(getSVCArgs...).Out.Contents()).To(ContainSubstring(serviceName))
				})
				When("the component is deleted using its name and namespace from another directory", func() {
					var out string
					BeforeEach(func() {
						otherDir := filepath.Join(commonVar.Context, "tmp")
						helper.MakeDir(otherDir)
						helper.Chdir(otherDir)
						out = helper.Cmd("odo", "delete", "component", "--name", cmpName, "--namespace", commonVar.Project, "-f").ShouldPass().Out()
					})

					It("should have deleted the component", func() {
						By("listing the resource to delete", func() {
							Expect(out).To(ContainSubstring("Deployment: " + deploymentName))
							Expect(out).To(ContainSubstring("Service: " + serviceName))
						})
						By("deleting the deployment", func() {
							Eventually(commonVar.CliRunner.Run(getDeployArgs...).Out.Contents(), 60, 3).ShouldNot(ContainSubstring(deploymentName))
						})
						By("deleting the service", func() {
							Eventually(commonVar.CliRunner.Run(getSVCArgs...).Out.Contents(), 60, 3).ShouldNot(ContainSubstring(serviceName))
						})
					})
				})

				for _, withFiles := range []bool{true, false} {
					withFiles := withFiles
					When(fmt.Sprintf("a resource is changed in the devfile and the component is deleted while having access to the devfile.yaml with --files=%v",
						withFiles), func() {
						var changedServiceName, stdout string
						BeforeEach(func() {
							changedServiceName = "my-changed-cs"
							helper.ReplaceString(filepath.Join(commonVar.Context, "devfile.yaml"), fmt.Sprintf("name: %s", serviceName), fmt.Sprintf("name: %s", changedServiceName))

							args := []string{"delete", "component", "-f"}
							if withFiles {
								args = append(args, "--files")
							}
							stdout = helper.Cmd("odo", args...).ShouldPass().Out()
						})
						It("should delete the component", func() {
							By("showing warning about undeleted service belonging to the component", func() {
								Expect(stdout).To(SatisfyAll(
									ContainSubstring("There are still resources left in the cluster that might be belonging to the deleted component"),
									Not(ContainSubstring(changedServiceName)),
									ContainSubstring(serviceName),
									ContainSubstring("odo delete component --name %s --namespace %s", cmpName, commonVar.Project),
								))
							})

							files := helper.ListFilesInDir(commonVar.Context)
							if withFiles {
								By("ensuring that devfile.yaml has been removed because it was created with odo init", func() {
									Expect(files).ShouldNot(ContainElement("devfile.yaml"))
								})
							} else {
								By("ensuring that devfile.yaml still exists", func() {
									Expect(files).To(ContainElement("devfile.yaml"))
								})
							}
						})

					})

					When("the component is deleted while having access to the devfile.yaml", func() {
						var stdOut string
						BeforeEach(func() {
							args := []string{"delete", "component", "-f"}
							if withFiles {
								args = append(args, "--files")
							}
							stdOut = helper.Cmd("odo", args...).ShouldPass().Out()
						})
						It("should have deleted the component", func() {
							By("listing the resources to delete", func() {
								Expect(stdOut).To(ContainSubstring(cmpName))
								Expect(stdOut).To(ContainSubstring("Deployment: " + deploymentName))
								Expect(stdOut).To(ContainSubstring("Service: " + serviceName))
							})
							By("deleting the deployment", func() {
								Eventually(commonVar.CliRunner.Run(getDeployArgs...).Out.Contents(), 60, 3).ShouldNot(ContainSubstring(deploymentName))
							})
							By("deleting the service", func() {
								Eventually(commonVar.CliRunner.Run(getSVCArgs...).Out.Contents(), 60, 3).ShouldNot(ContainSubstring(serviceName))
							})
							files := helper.ListFilesInDir(commonVar.Context)
							if withFiles {
								By("ensuring that devfile.yaml has been removed because it was created with odo init", func() {
									Expect(files).ShouldNot(ContainElement("devfile.yaml"))
								})
							} else {
								By("ensuring that devfile.yaml still exists", func() {
									Expect(files).To(ContainElement("devfile.yaml"))
								})
							}
						})
					})
				}

			})
		})
	}

	for _, withFiles := range []bool{true, false} {
		withFiles := withFiles
		When("deleting a component containing preStop event that is deployed with DEV and --files="+strconv.FormatBool(withFiles), func() {
			var out string
			BeforeEach(func() {
				// Hardcoded names from devfile-with-valid-events.yaml
				cmpName = "nodejs"
				helper.CopyExample(filepath.Join("source", "devfiles", "nodejs", "project"), commonVar.Context)
				helper.Cmd("odo", "init", "--name", cmpName, "--devfile-path", helper.GetExamplePath("source", "devfiles", "nodejs", "devfile-with-valid-events.yaml")).ShouldPass()
				session := helper.CmdRunner("odo", "dev", "--random-ports")
				defer session.Kill()
				helper.WaitForOutputToContain("[Ctrl+c] - Exit", 180, 10, session)
				// Ensure that the pod is in running state
				Eventually(string(commonVar.CliRunner.Run("get", "pods", "-n", commonVar.Project).Out.Contents()), 60, 3).Should(ContainSubstring(cmpName))
				// running in verbosity since the preStop events information is only printed in v4
				args := []string{"delete", "component", "-v", "4", "-f"}
				if withFiles {
					args = append(args, "--files")
				}
				out = helper.Cmd("odo", args...).ShouldPass().Out()
			})
			It("should delete the component", func() {
				By("listing preStop events", func() {
					helper.MatchAllInOutput(out, []string{
						"Executing myprestop command",
						"Executing secondprestop command",
						"Executing thirdprestop command",
					})
				})
				files := helper.ListFilesInDir(commonVar.Context)
				if withFiles {
					By("ensuring that appropriate files have been removed", func() {
						Expect(files).ShouldNot(ContainElement("devfile.yaml"))
					})
				} else {
					By("ensuring that devfile.yaml still exists", func() {
						Expect(files).To(ContainElement("devfile.yaml"))
					})
				}
			})
		})
	}
})
