package lxdbackend

import (
	"context"
	"fmt"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"

	"github.com/canonical/workshop/internal/workshop"
)

var (
	ConnectSimpleStreams = lxd.ConnectSimpleStreams
	imageServer          = "https://cloud-images.ubuntu.com/releases/"
)

func (b *Backend) Download(ctx context.Context, base string, report workshop.ProgressReporter) error {
	conn, err := b.LxdClient(ctx)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	// Check if we have the base image stored locally
	if _, _, err := conn.GetImageAlias(base); err == nil {
		if report != nil {
			report("download image", 1, 1)
		}
		return nil
	}

	names := strings.Split(base, "@")
	if len(names) <= 1 {
		return fmt.Errorf("%q base is not supported", base)
	}

	imageServer, err := ConnectSimpleStreams(imageServer, nil)
	if err != nil {
		return err
	}
	defer imageServer.Disconnect()

	var imageInfo *api.Image
	alias, _, err := imageServer.GetImageAlias(fmt.Sprintf("%s/%s", names[1], runtime.GOARCH))
	if err != nil {
		return err
	}

	imageInfo, _, err = imageServer.GetImage(alias.Target)
	if err != nil {
		return err
	}

	copyArgs := lxd.ImageCopyArgs{
		AutoUpdate: true,
		Public:     false,
		Type:       "",
		Aliases: []api.ImageAlias{
			{
				Name:        base,
				Description: "Workshop base image",
			},
		},
	}

	op, err := conn.CopyImage(imageServer, *imageInfo, &copyArgs)
	if err != nil {
		return err
	}

	if report != nil {
		op.AddHandler(func(o api.Operation) {
			if o.Metadata == nil || imageInfo.Size <= 0 {
				return
			}
			// state.Task supports only int for done/total
			handleLaunchUpdate(o.Metadata, int(imageInfo.Size), report)
		})
	}

	return op.Wait()
}

var imgDownload = regexp.MustCompile(`^rootfs: (?P<done>[0-9]+)% (?P<speed>\([\w/\.]+\))$`)

// handleLaunchUpdate parses a LXD create instance operation metadata and
// reports the opeartion's progress if available. The LXD metadata is
// inconsistent between operations that handleLaunchUpdate accomodates by
// looking for specific progress labels. NOTE: There is no guarantee that the
// LXD's progress reporting formant won't change; this meta data parser is valid
// for LXD 5.21.
func handleLaunchUpdate(opmeta map[string]interface{}, imsize int, progress workshop.ProgressReporter) {
	for key, value := range opmeta {
		if key == "download_progress" {
			upd, ok := value.(string)
			if !ok {
				continue
			}

			if data := imgDownload.FindStringSubmatch(upd); len(data) == 3 {
				done, err := strconv.Atoi(data[1])
				if err != nil {
					// just in case, but this is ensured by the regex
					continue
				}
				// now, "done" is the percentage value. The state.Task progress
				// reporting expects to have bytes all the way up to the client
				// to calculate the download speed. Thus, covert percentages to
				// bytes.
				donebytes := imsize * done / 100
				progress("download base image", donebytes, imsize)
			}
		}
	}
}

func FakeImageServer(server string) func() {
	oldImageServer := imageServer
	imageServer = server
	return func() { imageServer = oldImageServer }
}
