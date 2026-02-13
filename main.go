package main

import (
	"flag"
	"strings"

	"go.uber.org/fx"

	"github.com/joshuarp/withdraw-api/internal/app"
)

var defaultBin string

func selectedModules(binValue string) []fx.Option {
	selected := strings.TrimSpace(strings.ToLower(binValue))

	switch selected {
	case "inquiry", "inqury":
		return []fx.Option{
			app.AuthModule(),
			app.InquiryModule(),
		}
	case "withdraw":
		return []fx.Option{
			app.WithdrawModule(),
		}
	default:
		return []fx.Option{
			app.AuthModule(),
			app.InquiryModule(),
			app.WithdrawModule(),
		}
	}
}

func main() {
	bin := flag.String("bin", defaultBin, "select module binary: inquiry|withdraw (default: all)")
	flag.Parse()

	app.New(*bin, selectedModules(*bin)...).Run()
}
