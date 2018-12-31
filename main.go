package main

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/gkawamoto/k8s-designer/nsplot"
	_ "github.com/gkawamoto/k8s-designer/statik"
	"github.com/gkawamoto/k8s-designer/ui"
)

func main() {
	var err error
	var rootCmd = &cobra.Command{
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			var w *ui.Window
			w, err = ui.New(nil)
			if err != nil {
				log.Fatal("program:", err)
			}
			var p *nsplot.PlotHandler
			p, err = nsplot.NewPlotHandler(w, args[0])
			if err != nil {
				log.Fatal(err)
			}
			p.Run()
		},
	}
	err = rootCmd.Execute()
	if err != nil {
		log.Fatal(err)
	}
}
