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

var carAboutImagesOnce sync.Once
var carAccountQR, carRewardQR ui.ImageSource

// carAbout keeps the upstream project and the car edition clearly attributed.
func (a *app) carAbout(c *ui.Context, p palette) {
	ui.Column(c).Padding(28, 24, 28, 24).Gap(16).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "MyGo Music").FontSize(24).FontWeight(700)
		ui.Text(c, T("Car edition")+" · "+mygo.App.Version()).FontSize(13).TextColor(p.muted)
		ui.Text(c, T("Ported from MyGo Music to Android car systems. Thanks to original author egoist and all contributors.")).FontSize(14).TextColor(p.muted).LineHeight(1.5).TextAlign(ui.Center)
		ui.Column(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if ui.Button(c.Key("about-upstream"), T("Original author's GitHub")).Clicked() {
				_ = mygo.Shell.OpenExternal("https://github.com/egoist/mygo-music")
			}
			if ui.Button(c.Key("about-source"), T("Car edition source code")).Clicked() {
				_ = mygo.Shell.OpenExternal("https://github.com/benabcben/mygo-music-n7")
			}
		})
		carReward(c, p)
	})
}

func carReward(c *ui.Context, p palette) {
	carAboutImagesOnce.Do(func() {
		carAccountQR, _ = ui.DecodeImage(carAccountImage)
		carRewardQR, _ = ui.DecodeImage(carRewardImage)
	})
	ui.Row(c).Gap(12).AlignItems(ui.Start).Justify(ui.Center).Children(func() {
		carAboutCode(c, carAccountQR, T("WeChat account"), p)
		carAboutCode(c, carRewardQR, T("WeChat support"), p)
	})
}

func carAboutCode(c *ui.Context, image ui.ImageSource, label string, p palette) {
	ui.Column(c).Gap(7).AlignItems(ui.Center).Children(func() {
		if image != nil {
			ui.Image(c, image).Size(150, 150).Fit(ui.Contain).Label(label)
		}
		ui.Text(c, label).FontSize(13).TextColor(p.muted)
	})
}
