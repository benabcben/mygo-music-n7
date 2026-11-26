package app

import (
	_ "embed"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

//go:embed car_about_assets/account.jpg
var carAccountImage []byte

//go:embed car_about_assets/reward.jpg
var carRewardImage []byte

//go:embed car_about_assets/icon.png
var carIconImage []byte

var carAboutImagesOnce sync.Once
var carAccountQR, carRewardQR, carAboutIcon ui.ImageSource

// carAbout keeps the upstream project and the car edition clearly attributed.
func (a *app) carAbout(c *ui.Context, p palette) {
	carAboutImagesOnce.Do(func() {
		carAccountQR, _ = ui.DecodeImage(carAccountImage)
		carRewardQR, _ = ui.DecodeImage(carRewardImage)
		carAboutIcon, _ = ui.DecodeImage(carIconImage)
	})
	ui.Column(c).Padding(28, 24, 28, 24).Gap(16).AlignItems(ui.Center).Children(func() {
		if carAboutIcon != nil {
			ui.Image(c, carAboutIcon).Size(64, 64).Fit(ui.Contain).Label("MyGo Music")
		}
		ui.Text(c, "MyGo Music").FontSize(24).FontWeight(700)
		ui.Text(c, T("Car edition")+" · "+mygo.App.Version()).FontSize(13).TextColor(p.muted)
		ui.Text(c, T("Ported from MyGo Music to Android car systems. Thanks to original author egoist and all contributors.")).FontSize(14).TextColor(p.muted).LineHeight(1.5).TextAlign(ui.Center)
		ui.Row(c).WidthPercent(100).Wrap().Gap(8).Justify(ui.Center).Children(func() {
			if ui.Button(c.Key("about-upstream"), T("Original author")).Width(140).Clicked() {
				_ = mygo.Shell.OpenExternal("https://github.com/egoist/mygo-music")
			}
			if ui.Button(c.Key("about-source"), T("Source code")).Width(140).Clicked() {
				_ = mygo.Shell.OpenExternal("https://github.com/benabcben/mygo-music-n7")
			}
			ui.Button(c.Key("about-feedback"), T("Feedback")).Width(140).Disabled(true)
			ui.Button(c.Key("about-guide"), T("User guide")).Width(140).Disabled(true)
		})
		carReward(c, p)
		ui.Text(c, T("Based on the open source MyGo Music · AGPL-3.0")).FontSize(12).TextColor(p.muted).TextAlign(ui.Center)
	})
}

func carReward(c *ui.Context, p palette) {
	ui.Row(c).Gap(12).AlignItems(ui.Start).Justify(ui.Center).Children(func() {
		carAboutCode(c, carAccountQR, T("WeChat account"), p)
		carAboutCode(c, carRewardQR, T("WeChat support"), p)
	})
}

func carAboutCode(c *ui.Context, image ui.ImageSource, label string, p palette) {
	ui.Column(c).Width(150).Padding(10, 10, 12, 10).Gap(7).Radius(14).Background(p.hover).AlignItems(ui.Center).Children(func() {
		if image != nil {
			ui.Image(c, image).Size(130, 130).Fit(ui.Contain).Label(label)
		}
		ui.Text(c, label).FontSize(13).TextColor(p.muted)
	})
}
