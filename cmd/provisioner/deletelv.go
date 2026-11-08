package main

import (
	"fmt"

	"github.com/urfave/cli/v2"
	"k8s.io/klog/v2"

	"github.com/harvester/csi-driver-lvm/pkg/lvm"
)

func deleteLVCmd() *cli.Command {
	return &cli.Command{
		Name: "deletelv",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:  flagLVName,
				Usage: "Required. Specify lv name.",
			},
			&cli.StringFlag{
				Name:  flagSrcVGName,
				Usage: "Required. Source VG name",
			},
			&cli.StringFlag{
				Name:  flagSrcType,
				Usage: "Required. Source type, can be either striped or dm-thin",
			},
		},
		Action: func(c *cli.Context) error {
			if err := deleteLV(c); err != nil {
				klog.Fatalf("Error deleting lv: %v", err)
				return err
			}
			return nil
		},
	}
}

func deleteLV(c *cli.Context) error {
	lvName := c.String(flagLVName)
	if lvName == "" {
		return fmt.Errorf("invalid empty flag %v", flagLVName)
	}
	vgName := c.String(flagSrcVGName)
	if vgName == "" {
		return fmt.Errorf("invalid empty flag %v", flagSrcVGName)
	}
	vgType := c.String(flagSrcType)
	if vgType == "" {
		return fmt.Errorf("invalid empty flag %v", flagSrcType)
	}

	klog.Infof("delete lv %s", lvName)

	output, err := lvm.RemoveLVSInVG(vgName, lvName)
	if err != nil {
		return fmt.Errorf("unable to delete lv: %w output:%s", err, output)
	}
	// The thin pool is shared with other volumes and concurrent helpers. Even
	// an empty pool can acquire a new volume between an lvs check and lvremove,
	// so volume deletion must leave the pool in place for reuse.
	klog.Infof("lv %s is deleted", lvName)
	return nil
}
