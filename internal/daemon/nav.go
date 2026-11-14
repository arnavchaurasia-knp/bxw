package daemon

import (
	"context"
	"time"

	"github.com/chromedp/cdproto/cdp"
	"github.com/chromedp/cdproto/page"

	"bxw/internal/browser"
)

func (d *Daemon) gotoURL(tabID, targetURL string, timeout time.Duration) (ActResult, error) {
	if err := browser.CheckURL(targetURL); err != nil {
		return ActResult{}, err
	}

	return d.actOn(tabID, timeout, func(ctx context.Context, sess *tabSession) error {
		_, err := cdp.Call(ctx, sess.conn, page.Navigate, page.NavigateParams{URL: targetURL})
		return err
	})
}
