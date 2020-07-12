// This file defines the helpers to develop automation.
// Such as when running automation we can use trace to visually
// see where the mouse going to click.

package rod

import (
	"fmt"
	"html"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/go-rod/rod/lib/assets"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"github.com/ysmood/kit"
)

// check method and sleep if needed
func (b *Browser) trySlowmotion() {
	if b.slowmotion == 0 {
		return
	}

	time.Sleep(b.slowmotion)
}

func (el *Element) tryTrace(htmlMessage string) func() {
	if !el.page.browser.trace {
		return func() {}
	}

	if !el.page.browser.quiet {
		kit.Log(kit.C("[trace]", "cyan"), htmlMessage)
	}

	return el.Trace(htmlMessage)
}

// ServeMonitor starts the monitor server.
// If openBrowser is true, it will try to launcher a browser to play the screenshots.
// The reason why not to use "chrome://inspect/#devices" is one target cannot be driven by multiple controllers.
func (b *Browser) ServeMonitor(host string, openBrowser bool) *kit.ServerContext {
	if host == "" {
		return nil
	}

	srv := kit.MustServer(host)
	opts := &http.Server{}
	opts.SetKeepAlivesEnabled(false)
	srv.Set(opts)

	srv.Engine.Use(func(ctx kit.GinContext) {
		defer func() {
			if err := recover(); err != nil {
				_ = ctx.AbortWithError(400, fmt.Errorf("%v", err))
			}
		}()
		ctx.Next()
	})
	srv.Engine.GET("/", func(ctx kit.GinContext) {
		ginHTML(ctx, assets.Monitor)
	})
	srv.Engine.GET("/pages", func(ctx kit.GinContext) {
		res, err := proto.TargetGetTargets{}.Call(b)
		kit.E(err)
		ctx.PureJSON(http.StatusOK, res.TargetInfos)
	})
	srv.Engine.GET("/page/:id", func(ctx kit.GinContext) {
		ginHTML(ctx, assets.MonitorPage)
	})
	srv.Engine.GET("/api/page/:id", func(ctx kit.GinContext) {
		info, err := b.pageInfo(proto.TargetTargetID(ctx.Param("id")))
		kit.E(err)
		ctx.PureJSON(http.StatusOK, info)
	})
	srv.Engine.GET("/screenshot/:id", func(ctx kit.GinContext) {
		id := proto.TargetTargetID(ctx.Param("id"))
		p := b.PageFromTargetID(id)

		ctx.Header("Content-Type", "image/png;")
		_, _ = ctx.Writer.Write(p.Screenshot())
	})

	var viewer *Browser

	go func() { _ = srv.Do() }()
	go func() {
		<-b.ctx.Done()
		_ = srv.Listener.Close()
		if openBrowser {
			_ = viewer.CloseE()
		}
	}()

	url := "http://" + srv.Listener.Addr().String()

	if openBrowser {
		viewer = New().Context(b.ctx, b.ctxCancel).ControlURL(
			launcher.New().Headless(false).Launch(),
		).Connect()
		viewer.Page(url)
	}

	return srv
}

// Overlay a rectangle on the main frame with specified message
func (p *Page) Overlay(left, top, width, height float64, msg string) (remove func()) {
	root := p.Root()
	id := kit.RandString(8)

	js, jsArgs := p.jsHelper("overlay", Array{
		id,
		left,
		top,
		width,
		height,
		msg,
	})
	_, err := root.EvalE(true, "", js, jsArgs)
	CancelPanic(err)

	remove = func() {
		js, jsArgs := p.jsHelper("removeOverlay", Array{id})
		_, _ = root.EvalE(true, "", js, jsArgs)
	}

	return
}

// Trace with an overlay on the element
func (el *Element) Trace(htmlMessage string) (removeOverlay func()) {
	id := kit.RandString(8)

	js, jsArgs := el.page.jsHelper("elementOverlay", Array{
		id,
		htmlMessage,
	})
	_, err := el.EvalE(true, js, jsArgs)
	CancelPanic(err)

	removeOverlay = func() {
		js, jsArgs := el.page.jsHelper("removeOverlay", Array{id})
		_, _ = el.EvalE(true, js, jsArgs)
	}

	return
}

var regFn = regexp.MustCompile(`\A\(rod, \.\.\.args\) => (rod\..+)\.apply\(this, `)

func (p *Page) tryTraceFn(js string, params Array) func() {
	if !p.browser.trace {
		return func() {}
	}

	matches := regFn.FindStringSubmatch(js)
	if matches != nil {
		js = matches[1]
		params = params[1:]
	}
	paramsStr := strings.Trim(mustToJSONForDev(params), "[]\r\n")

	if !p.browser.quiet {
		msg := fmt.Sprintf("%s(%s)", js, paramsStr)
		kit.Log(kit.C("[trace]", "cyan"), kit.C("js", "yellow"), msg)
	}

	msg := fmt.Sprintf("js <code>%s(%s)</code>", js, html.EscapeString(paramsStr))
	return p.Overlay(0, 0, 500, 0, msg)
}
