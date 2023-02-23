package main

import (
	"fmt"
	"math/rand"
	"os"

	crypto_rand "crypto/rand"
	"encoding/binary"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:              "workspace",
	SilenceErrors:    false,
	SilenceUsage:     true,
	TraverseChildren: true,
}

var Project string

func init() {

}

func init() {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Println(err)
		panic("cannot get a current working directory")
	}

	var b [8]byte
	_, err = crypto_rand.Read(b[:])
	if err != nil {
		panic("cannot seed math/rand package")
	}
	rand.Seed(int64(binary.LittleEndian.Uint64(b[:])))

	rootCmd.PersistentFlags().StringVarP(&Project, "project", "p", cwd, "specify a project's directory path")

	rootCmd.AddCommand((&CmdLaunch{}).Command())
	rootCmd.AddCommand((&CmdList{}).Command())
}

func main() {
	rootCmd.Execute()
}
