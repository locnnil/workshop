package main

import (
	"log"
	"os"

	"github.com/canonical/gencodo"
	"github.com/spf13/cobra"

	"github.com/canonical/workshop/cmd/internal/doctemplates"
)

type CmdDocs struct {
	root *CmdRoot
}

func (c *CmdDocs) Command() *cobra.Command {
	var cmd = &cobra.Command{
		Use:    "generate-docs",
		Args:   cobra.MaximumNArgs(1),
		Short:  "Generate sdk reference docs",
		Hidden: true,
		RunE:   c.Run,
	}

	return cmd
}

func filePrepender(filename string) string {
	return ""
}

func (c *CmdDocs) Run(cmd *cobra.Command, av []string) error {
	docDir := "docs-gendocs"
	if len(av) > 0 {
		docDir = av[0]
	}

	err := os.MkdirAll(docDir, os.ModePerm)
	if err != nil {
		log.Fatalf("failed to create docs directory: %v", err)
	}

	indexTemplate, err := doctemplates.ReadFile("sdk.rst")
	if err != nil {
		return err
	}
	singleCommandTemplate, err := doctemplates.ReadFile("command.rst")
	if err != nil {
		return err
	}

	td := gencodo.TemplateInfo{
		IndexFileName:         "sdk.rst",
		IndexTemplate:         string(indexTemplate),
		SingleCommandTemplate: string(singleCommandTemplate),
	}

	err = gencodo.GenRSTTree(
		c.root.Command(),
		docDir,
		td,
		filePrepender,
	)
	if err != nil {
		log.Fatalf("failed to generate documentation: %v", err)
	}
	return nil
}
