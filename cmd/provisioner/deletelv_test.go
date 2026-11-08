package main

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/harvester/csi-driver-lvm/pkg/lvm"
	"github.com/urfave/cli/v2"
)

func TestDeleteLVPreservesSharedPool(t *testing.T) {
	for _, tc := range []struct {
		name       string
		volumeType string
		absent     bool
		removeFail bool
	}{
		{name: "last thin volume", volumeType: lvm.DmThinType},
		{name: "already absent thin volume", volumeType: lvm.DmThinType, absent: true},
		{name: "striped volume", volumeType: lvm.StripedType},
		{name: "failed volume removal", volumeType: lvm.DmThinType, removeFail: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			callsPath := filepath.Join(dir, "calls")
			t.Setenv("PATH", dir)
			t.Setenv("LVM_TEST_CALLS", callsPath)
			t.Setenv("LVM_TEST_REMOVE_FAIL", "0")
			if tc.removeFail {
				t.Setenv("LVM_TEST_REMOVE_FAIL", "1")
			}
			volumes := `{"lv_name":"vg-thinpool","vg_name":"vg","lv_size":"10485760","segtype":"thin-pool"}`
			if !tc.absent {
				volumes += `,{"lv_name":"volume","vg_name":"vg","lv_size":"1048576","segtype":"thin"}`
			}
			t.Setenv("LVM_TEST_REPORT", `{"report":[{"lv":[`+volumes+`]}]}`)

			// Use fake executables to exercise the real delete helper through its
			// command boundary without accessing host LVM devices. The old cleanup
			// path sees an empty pool and attempts a forbidden pool removal.
			commands := map[string]string{
				"lvs": `#!/bin/sh
printf 'lvs %s\n' "$*" >> "$LVM_TEST_CALLS"
case "$*" in
  *thin_count*) printf 'vg thin-pool vg-thinpool 0\n' ;;
  *) printf '%s\n' "$LVM_TEST_REPORT" ;;
esac
`,
				"lvremove": `#!/bin/sh
printf 'lvremove %s\n' "$*" >> "$LVM_TEST_CALLS"
if [ "$*" != '-q -y vg/volume' ]; then
  echo 'refusing to remove a shared pool' >&2
  exit 1
fi
if [ "$LVM_TEST_REMOVE_FAIL" = 1 ]; then
  echo 'volume is busy' >&2
  exit 1
fi
`,
			}
			for name, script := range commands {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0o700); err != nil { //nolint:gosec // Private test command stubs must be executable.
					t.Fatal(err)
				}
			}

			flags := flag.NewFlagSet("deletelv", flag.ContinueOnError)
			flags.String(flagLVName, "volume", "")
			flags.String(flagSrcVGName, "vg", "")
			flags.String(flagSrcType, tc.volumeType, "")
			err := deleteLV(cli.NewContext(cli.NewApp(), flags, nil))
			if tc.removeFail {
				if err == nil || !strings.Contains(err.Error(), "volume is busy") {
					t.Fatalf("expected volume removal error, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("deleteLV failed: %v", err)
			}

			calls, err := os.ReadFile(callsPath)
			if err != nil {
				t.Fatal(err)
			}
			want := "lvs --reportformat json --units b --nosuffix --options lv_name,vg_name,lv_size,segtype,origin\n"
			if !tc.absent {
				want += "lvremove -q -y vg/volume\n"
			}
			if string(calls) != want {
				t.Fatalf("volume deletion must only inspect and remove its target LV; want %q, got %q", want, calls)
			}
		})
	}
}
