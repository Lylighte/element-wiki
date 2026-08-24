package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestSessionCookieAttributes(t *testing.T) {
	rec := httptest.NewRecorder()
	setSessionCookie(rec, cookieCfg{secureCookies: true}, "raw-value", 123, 3600)
	cookies := rec.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatal("应写出一个 cookie")
	}
	c := cookies[0]
	if c.Name != "ew_session" || c.Value != "raw-value" || !c.HttpOnly || !c.Secure {
		t.Errorf("cookie 属性异常: %+v", c)
	}
	if c.MaxAge != 3600 || c.Path != "/" {
		t.Errorf("maxage/path 异常: %+v", c)
	}

	rec2 := httptest.NewRecorder()
	clearSessionCookie(rec2, cookieCfg{secureCookies: false})
	c2 := rec2.Result().Cookies()[0]
	if c2.MaxAge != -1 || c2.Value != "" || c2.Secure {
		t.Errorf("清除 cookie 异常: %+v", c2)
	}
}
