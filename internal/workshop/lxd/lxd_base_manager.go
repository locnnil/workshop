package lxdbackend

import (
	"context"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"

	lxd "github.com/canonical/lxd/client"
	"github.com/canonical/lxd/shared/api"
	"github.com/canonical/lxd/shared/units"

	"github.com/canonical/workshop/internal/dirs"
	"github.com/canonical/workshop/internal/logger"
	"github.com/canonical/workshop/internal/osutil"
	"github.com/canonical/workshop/internal/progress"
)

var (
	ConnectSimpleStreams = lxd.ConnectSimpleStreams
	imageServer          = "simplestreams:https://cloud-images.ubuntu.com/releases"
)

func (b *Backend) Download(ctx context.Context, base string, report *progress.Reporter) error {
	defer func() {
		if report != nil {
			imageLock.Lock()
			if op, exist := currentDownloads[base]; exist {
				op.RemoveReporter(report.Name)
			}
			imageLock.Unlock()
		}
	}()

	imageLock.Lock()
	op, exist := currentDownloads[base]
	if exist {
		if report != nil {
			op.AddReporter(report)
		}
		imageLock.Unlock()
		return waitDownloadOp(ctx, op)
	}

	op = newImageDownloadOp()
	if report != nil {
		op.AddReporter(report)
	}
	currentDownloads[base] = op
	imageLock.Unlock()

	go b.download(ctx, op, base)

	return waitDownloadOp(ctx, op)
}

func (b *Backend) download(ctx context.Context, op *downloadOp, base string) (err error) {
	defer func() {
		op.waitCh <- err
		close(op.waitCh)

		imageLock.Lock()
		delete(currentDownloads, base)
		imageLock.Unlock()
	}()

	// LXD cannot cancel download operations
	child := context.WithoutCancel(ctx)

	conn, err := b.LxdClient(child)
	if err != nil {
		return err
	}
	defer conn.Disconnect()

	alias, _, err := conn.GetImageAlias(ImageAlias(base))
	if err == nil {
		logger.Debugf("BaseImageManager on Download: %q image already exists (%s)", alias.Name, alias.Target)
		return
	}

	names := strings.FieldsFunc(base, func(r rune) bool { return r == '@' })
	if len(names) != 2 {
		return fmt.Errorf("%q is not a correct base name", base)
	}

	imageServer, err := connectImageServer(imageServer)
	if err != nil {
		return err
	}
	defer imageServer.Disconnect()

	var imageInfo *api.Image
	alias, _, err = imageServer.GetImageAlias(fmt.Sprintf("%s/%s", names[1], runtime.GOARCH))
	if err != nil {
		return fmt.Errorf("%q download failed: %w", base, err)
	}

	imageInfo, _, err = imageServer.GetImage(alias.Target)
	if err != nil {
		return fmt.Errorf("%q download failed: %w", base, err)
	}

	copyArgs := lxd.ImageCopyArgs{
		AutoUpdate: true,
		Public:     false,
		Type:       "container",
	}

	copyop, err := conn.CopyImage(imageServer, *imageInfo, &copyArgs)
	if err != nil {
		return fmt.Errorf("%q download failed: %w", base, err)
	}

	copyop.AddHandler(func(o api.Operation) {
		if o.Metadata == nil || imageInfo.Size <= 0 {
			return
		}
		if upd := handleLaunchUpdate(o.Metadata, int(imageInfo.Size)); upd != nil {
			op.Update(*upd)
		}
	})

	if err = copyop.Wait(); err == nil {
		// The LXD image alias must be updated separately as if provided to CopyImage in
		// multiple concurrent calls it will fail with "Alias already exists" once the
		// image is downloaded. This happens because LXD's CopyImage handles image
		// download and creating an alias separately and not as a single transaction.
		b.maybeUpdateAlias(conn, base, imageInfo.Fingerprint)
	}

	return err
}

// For the test purposes only
func lxdConnectionArgs() (*lxd.ConnectionArgs, error) {
	args := &lxd.ConnectionArgs{}

	// Server certificate
	scrt := filepath.Join(dirs.WorkshopTlsDir, "server.crt")
	if osutil.FileExists(scrt) {
		content, err := os.ReadFile(scrt)
		if err != nil {
			return nil, err
		}

		args.TLSServerCert = string(content)
	}

	// Client certificate
	ccrt := filepath.Join(dirs.WorkshopTlsDir, "client.crt")
	if osutil.FileExists(ccrt) {
		content, err := os.ReadFile(ccrt)
		if err != nil {
			return nil, err
		}

		args.TLSClientCert = string(content)
	}

	// Client CA
	cca := filepath.Join(dirs.WorkshopTlsDir, "client.ca")
	if osutil.FileExists(cca) {
		content, err := os.ReadFile(cca)
		if err != nil {
			return nil, err
		}

		args.TLSCA = string(content)
	}

	// Client key
	ckey := filepath.Join(dirs.WorkshopTlsDir, "client.key")
	if osutil.FileExists(ckey) {
		content, err := os.ReadFile(ckey)
		if err != nil {
			return nil, err
		}

		pemKey, _ := pem.Decode(content)
		// Golang has deprecated all methods relating to PEM encryption due to a vulnerability.
		// However, the weakness does not make PEM unsafe for our purposes as it pertains to password protection on the
		// key file (client.key is only readable to the user in any case), so we'll ignore deprecation.
		if x509.IsEncryptedPEMBlock(pemKey) { //nolint:staticcheck
			return nil, fmt.Errorf("private key is password protected and no helper was configured")
		}

		args.TLSClientKey = string(content)
	}
	return args, nil
}

func connectImageServer(url string) (lxd.ImageServer, error) {
	if strings.HasPrefix(url, "simplestreams:") {
		server, _ := strings.CutPrefix(url, "simplestreams:")
		conn, err := ConnectSimpleStreams(server, nil)
		if err != nil {
			return nil, fmt.Errorf("image server is not available: %w", err)
		}
		return conn, err
	}

	if strings.HasPrefix(url, "lxd:") {
		server, _ := strings.CutPrefix(url, "lxd:")
		args, err := lxdConnectionArgs()
		if err != nil {
			return nil, err
		}
		conn, err := lxd.ConnectPublicLXD(server, args)
		if err != nil {
			return nil, fmt.Errorf("image server is not available: %w", err)
		}
		return conn, err
	}

	return nil, fmt.Errorf("unknown image server URL prefix (supported: simplestreams, lxd)")
}

func waitDownloadOp(ctx context.Context, op *downloadOp) error {
	select {
	case <-ctx.Done():
		// Do not try to cancel the target op here as LXD is unable to cancel
		// image download properly. Instead, we'll wait for it to finish if
		// the task will be restarted.
		return ctx.Err()
	case err := <-op.waitCh:
		return err
	}
}

func (b *Backend) maybeUpdateAlias(conn lxd.InstanceServer, base, fingerprint string) {
	alias := api.ImageAliasesPost{}
	alias.Target = fingerprint
	alias.Name = ImageAlias(base)

	_, _, err := conn.GetImageAlias(alias.Name)
	if api.StatusErrorCheck(err, http.StatusNotFound) {
		if err = conn.CreateImageAlias(alias); err != nil {
			logger.Noticef("BaseImageManager on Download: Failed to create an alias %q for %q base: %v", alias.Name, base, err)
		}
	}
}

var (
	imgDownloadSS = regexp.MustCompile(`^rootfs: (?P<done>[0-9]+)% (?P<speed>\([\w/\.]+\))$`)
	// 225.19MB (37.46MB/s)
	imgDownloadLXD = regexp.MustCompile(`^(?P<done>[0-9\.]+)(?P<mult>\w+) (?P<speed>\([\w/\.]+\))$`)
)

// handleLaunchUpdate parses a LXD create instance operation metadata and
// reports the opeartion's progress if available. The LXD metadata is
// inconsistent between operations that handleLaunchUpdate accomodates by
// looking for specific progress labels. NOTE: There is no guarantee that the
// LXD's progress reporting formant won't change; this meta data parser is valid
// for LXD 5.21.
func handleLaunchUpdate(opmeta map[string]interface{}, imsize int) *downloadUpdate {
	for key, value := range opmeta {
		if key == "download_progress" {
			upd, ok := value.(string)
			if !ok {
				continue
			}

			// check if the response metadata comes from a simplestream protocol
			if data := imgDownloadSS.FindStringSubmatch(upd); len(data) == 3 {
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
				return &downloadUpdate{Label: "download", Done: donebytes, Total: imsize}
			}

			// check if the response metadata comes from a lxd protocol
			if data := imgDownloadLXD.FindStringSubmatch(upd); len(data) == 4 {
				done, err := strconv.ParseFloat(data[1], 32)
				if err != nil {
					continue
				}
				// ParseByteSizeString understands only int, so we use it to get
				// a multiplier for "done".
				multiplier, err := units.ParseByteSizeString("1" + data[2])
				if err != nil {
					continue
				}
				donebytes := int(done) * int(multiplier)
				return &downloadUpdate{Label: "download", Done: donebytes, Total: imsize}
			}
		}
	}
	return nil
}

type downloadUpdate struct {
	Label string
	Done  int
	Total int
}

type downloadOp struct {
	waitCh chan error

	reportersLock sync.Mutex
	reporters     map[string]*progress.Reporter
}

func newImageDownloadOp() *downloadOp {
	return &downloadOp{waitCh: make(chan error), reporters: make(map[string]*progress.Reporter, 0)}
}

func (r *downloadOp) AddReporter(rep *progress.Reporter) {
	r.reportersLock.Lock()
	defer r.reportersLock.Unlock()

	r.reporters[rep.Name] = rep
}

func (r *downloadOp) RemoveReporter(name string) {
	r.reportersLock.Lock()
	defer r.reportersLock.Unlock()
	delete(r.reporters, name)
}

func (r *downloadOp) Update(upd downloadUpdate) {
	r.reportersLock.Lock()
	defer r.reportersLock.Unlock()

	for _, rep := range r.reporters {
		rep.Report(upd.Label, upd.Done, upd.Total)
	}
}

func FakeImageServer(server string) func() {
	oldImageServer := imageServer
	imageServer = server
	return func() { imageServer = oldImageServer }
}
