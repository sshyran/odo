package devfile

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"

	devfiletesting "github.com/redhat-developer/odo/pkg/devfile/testing"

	devfilev1 "github.com/devfile/api/v2/pkg/apis/workspaces/v1alpha2"
	"github.com/devfile/library/v2/pkg/devfile/parser"
	devfileCtx "github.com/devfile/library/v2/pkg/devfile/parser/context"
	devfileFileSystem "github.com/devfile/library/v2/pkg/testingutil/filesystem"
)

func TestGetKubernetesComponentsToPush(t *testing.T) {
	fs := devfileFileSystem.NewFakeFs()

	getDevfileWithoutApplyCommand := func() parser.DevfileObj {
		devfileObj := parser.DevfileObj{
			Data: devfiletesting.GetDevfileData(t, []devfiletesting.InlinedComponent{
				{
					Name:    "component1",
					Inlined: "Component 1",
				},
			}, nil),
			Ctx: devfileCtx.FakeContext(fs, parser.OutputDevfileYamlPath),
		}
		return devfileObj
	}

	getDevfileWithApplyCommand := func(applyComponentName string) parser.DevfileObj {
		const defaultComponentName = "component1"
		devfileObj := parser.DevfileObj{
			Data: devfiletesting.GetDevfileData(t, []devfiletesting.InlinedComponent{
				{
					Name:    defaultComponentName,
					Inlined: "Component 1",
				},
			}, nil),
			Ctx: devfileCtx.FakeContext(fs, parser.OutputDevfileYamlPath),
		}
		applyCommand := devfilev1.Command{
			CommandUnion: devfilev1.CommandUnion{
				Apply: &devfilev1.ApplyCommand{
					Component: applyComponentName,
				},
			},
		}
		if applyComponentName != defaultComponentName {
			_ = devfileObj.Data.AddComponents([]devfilev1.Component{
				{
					Name: applyComponentName,
					ComponentUnion: devfilev1.ComponentUnion{
						Kubernetes: &devfilev1.KubernetesComponent{
							K8sLikeComponent: devfilev1.K8sLikeComponent{
								K8sLikeComponentLocation: devfilev1.K8sLikeComponentLocation{
									Inlined: applyComponentName,
								},
							},
						},
					},
				},
			})
		}
		_ = devfileObj.Data.AddCommands([]devfilev1.Command{applyCommand})
		return devfileObj
	}

	tests := []struct {
		name       string
		devfileObj parser.DevfileObj
		want       []devfilev1.Component
		allowApply bool
		wantErr    bool
	}{
		{
			name: "empty devfile",
			devfileObj: parser.DevfileObj{
				Data: devfiletesting.GetDevfileData(t, nil, nil),
				Ctx:  devfileCtx.FakeContext(fs, parser.OutputDevfileYamlPath),
			},
			want:    []devfilev1.Component{},
			wantErr: false,
		},
		{
			name:       "no apply command",
			devfileObj: getDevfileWithoutApplyCommand(),
			want: []devfilev1.Component{
				{
					Name: "component1",
					ComponentUnion: devfilev1.ComponentUnion{
						Kubernetes: &devfilev1.KubernetesComponent{
							K8sLikeComponent: devfilev1.K8sLikeComponent{
								K8sLikeComponentLocation: devfilev1.K8sLikeComponentLocation{
									Inlined: "Component 1",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:       "apply command referencing the component",
			devfileObj: getDevfileWithApplyCommand("component1"),
			want:       []devfilev1.Component{},
			wantErr:    false,
		},
		{
			name:       "apply command not referencing the component",
			devfileObj: getDevfileWithApplyCommand("other component"),
			want: []devfilev1.Component{
				{
					Name: "component1",
					ComponentUnion: devfilev1.ComponentUnion{
						Kubernetes: &devfilev1.KubernetesComponent{
							K8sLikeComponent: devfilev1.K8sLikeComponent{
								K8sLikeComponentLocation: devfilev1.K8sLikeComponentLocation{
									Inlined: "Component 1",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name:       "allow component referenced by apply command when allowApply is true",
			devfileObj: getDevfileWithApplyCommand("component2"),
			allowApply: true,
			want: []devfilev1.Component{
				{
					Name: "component1",
					ComponentUnion: devfilev1.ComponentUnion{
						Kubernetes: &devfilev1.KubernetesComponent{
							K8sLikeComponent: devfilev1.K8sLikeComponent{
								K8sLikeComponentLocation: devfilev1.K8sLikeComponentLocation{
									Inlined: "Component 1",
								},
							},
						},
					},
				},
				{
					Name: "component2",
					ComponentUnion: devfilev1.ComponentUnion{
						Kubernetes: &devfilev1.KubernetesComponent{
							K8sLikeComponent: devfilev1.K8sLikeComponent{
								K8sLikeComponentLocation: devfilev1.K8sLikeComponentLocation{
									Inlined: "component2",
								},
							},
						},
					},
				},
			},
			wantErr: false,
		},
	}

	lessFunc := func(x, y devfilev1.Component) bool {
		return x.Name < y.Name
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetKubernetesComponentsToPush(tt.devfileObj, tt.allowApply)
			gotErr := err != nil
			if gotErr != tt.wantErr {
				t.Errorf("Got error %v, expected %v\n", err, tt.wantErr)
			}

			if len(got) != len(tt.want) {
				t.Errorf("Got %d components, expected %d\n", len(got), len(tt.want))
			}

			if diff := cmp.Diff(tt.want, got, cmpopts.SortSlices(lessFunc)); diff != "" {
				t.Errorf("GetKubernetesComponentsToPush() mismatch (-want +got):\n%s", diff)
			}
		})
	}
}
