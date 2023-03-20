package main

import (
	"fmt"

	"github.com/powerpuffpenguin/httpadapter/core"
	"github.com/spf13/cobra"
)

func root() *cobra.Command {
	var (
		version, protocol bool
	)
	cmd := &cobra.Command{
		Use:   "httpadapter",
		Short: "an http adapter",
		Run: func(cmd *cobra.Command, args []string) {
			var i = 0
			if version {
				fmt.Println(core.Version)
				i++
			}
			if protocol {
				fmt.Println(core.ProtocolVersion)
				i++
			}
			if i != 0 {
				return
			}
			fmt.Println(`an http adapter `)
			fmt.Println(`protocol`, core.ProtocolVersion)
			fmt.Println(`version`, core.Version)
		},
	}
	flags := cmd.Flags()
	flags.BoolVarP(&version, `version`, `v`, false, `print version`)
	flags.BoolVarP(&protocol, `protocol`, `p`, false, `print protocol version`)
	return cmd
}
