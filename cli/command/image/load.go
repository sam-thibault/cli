package image

import (
	"context"
	"io"

	"github.com/containerd/platforms"
	"github.com/docker/cli/cli"
	"github.com/docker/cli/cli/command"
	"github.com/docker/cli/cli/command/completion"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/moby/sys/sequential"
	"github.com/pkg/errors"
	"github.com/spf13/cobra"
)

type loadOptions struct {
	input    string
	quiet    bool
	platform string
}

// NewLoadCommand creates a new `docker load` command
func NewLoadCommand(dockerCli command.Cli) *cobra.Command {
	var opts loadOptions

	cmd := &cobra.Command{
		Use:   "load [OPTIONS]",
		Short: "Load an image from a tar archive or STDIN",
		Args:  cli.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLoad(cmd.Context(), dockerCli, opts)
		},
		Annotations: map[string]string{
			"aliases": "docker image load, docker load",
		},
		ValidArgsFunction: completion.NoComplete,
	}

	flags := cmd.Flags()

	flags.StringVarP(&opts.input, "input", "i", "", "Read from tar archive file, instead of STDIN")
	flags.BoolVarP(&opts.quiet, "quiet", "q", false, "Suppress the load output")
	flags.StringVar(&opts.platform, "platform", "", `Load only the given platform variant. Formatted as "os[/arch[/variant]]" (e.g., "linux/amd64")`)
	_ = flags.SetAnnotation("platform", "version", []string{"1.48"})

	_ = cmd.RegisterFlagCompletionFunc("platform", completion.Platforms)
	return cmd
}

func runLoad(ctx context.Context, dockerCli command.Cli, opts loadOptions) error {
	var input io.Reader = dockerCli.In()
	if opts.input != "" {
		// We use sequential.Open to use sequential file access on Windows, avoiding
		// depleting the standby list un-necessarily. On Linux, this equates to a regular os.Open.
		file, err := sequential.Open(opts.input)
		if err != nil {
			return err
		}
		defer file.Close()
		input = file
	}

	// To avoid getting stuck, verify that a tar file is given either in
	// the input flag or through stdin and if not display an error message and exit.
	if opts.input == "" && dockerCli.In().IsTerminal() {
		return errors.Errorf("requested load from stdin, but stdin is empty")
	}

	var options image.LoadOptions
	if opts.quiet || !dockerCli.Out().IsTerminal() {
		options.Quiet = true
	}

	if opts.platform != "" {
		p, err := platforms.Parse(opts.platform)
		if err != nil {
			return errors.Wrap(err, "invalid platform")
		}
		options.Platform = &p
	}

	response, err := dockerCli.Client().ImageLoad(ctx, input, options)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.Body != nil && response.JSON {
		return jsonmessage.DisplayJSONMessagesToStream(response.Body, dockerCli.Out(), nil)
	}

	_, err = io.Copy(dockerCli.Out(), response.Body)
	return err
}
