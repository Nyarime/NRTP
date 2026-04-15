package nrtp

import (
	"bufio"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// PortalServeHTTP 在TLS连接上serve HTTP Portal + XML认证
func PortalServeHTTP(conn net.Conn) {
	defer conn.Close()
	br := bufio.NewReader(conn)
	for {
		req, err := http.ReadRequest(br)
		if err != nil {
			return
		}
		req.Body.Close()

		path := req.URL.Path
		ua := req.Header.Get("User-Agent")
		isAnyConnect := strings.Contains(ua, "AnyConnect") || 
			req.Header.Get("X-Transcend-Version") != "" ||
			req.Header.Get("X-Aggregate-Auth") != ""

		var body string
		var contentType string

		if isAnyConnect && req.Method == "POST" {
			// AnyConnect XML认证
			contentType = "text/xml; charset=utf-8"
			body = generateAuthRequestXML()
		} else if path == "/+CSCOE+/logon.html" || path == "/" {
			contentType = "text/html; charset=utf-8"
			body = generatePortalJS()
		} else if strings.HasPrefix(path, "/CSCOSSLC/tunnel") {
			body = "SSL VPN session required"
			contentType = "text/plain"
			writeHTTPResponse(conn, 403, contentType, body)
			return
		} else {
			contentType = "text/html; charset=utf-8"
			body = portalHTML
		}

		writeHTTPResponse(conn, 200, contentType, body)

		if req.Header.Get("Connection") == "close" {
			return
		}
	}
}

func generatePortalJS() string {
	return `<html><script>
document.cookie = "tg=; expires=Thu, 01 Jan 1970 22:00:00 GMT; path=/; secure";
document.cookie = "sdesktop=; expires=Thu, 01 Jan 1970 22:00:00 GMT; path=/; secure";
document.location.replace("/+CSCOE+/logon.html");
</script></html>`
}

func generateAuthRequestXML() string {
	group := "Employee-SSO"
	configHash := time.Now().Unix()
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<config-auth client="vpn" type="auth-request" aggregate-auth-version="2">
<opaque is-for="sg">
<tunnel-group>%s</tunnel-group>
<group-alias>%s</group-alias>
<config-hash>%d</config-hash>
</opaque>
<auth id="main">
<title>Login</title>
<message>Please enter your username and password.</message>
<banner></banner>
<form>
<input type="text" name="username" label="Username:"></input>
<input type="password" name="password" label="Password:"></input>
<select name="group_list" label="GROUP:">
<option selected="true">%s</option>
</select>
</form>
</auth>
</config-auth>`, group, group, configHash, group)
}

// GenerateSessionToken 生成AnyConnect session token
func GenerateSessionToken() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func writeHTTPResponse(conn net.Conn, status int, contentType, body string) {
	statusText := "OK"
	if status == 403 { statusText = "Forbidden" }
	
	expired := "Thu, 01 Jan 1970 22:00:00 GMT"
	cookies := ""
	for _, name := range []string{"webvpn", "webvpnc", "webvpn_portal", "webvpnlogin"} {
		if name == "webvpnlogin" {
			cookies += fmt.Sprintf("Set-Cookie: %s=1; path=/; secure\r\n", name)
		} else {
			cookies += fmt.Sprintf("Set-Cookie: %s=; expires=%s; path=/; secure\r\n", name, expired)
		}
	}
	
	resp := fmt.Sprintf("HTTP/1.1 %d %s\r\n"+
		"Server: Cisco ASDM\r\n"+
		"Content-Type: %s\r\n"+
		"Content-Length: %d\r\n"+
		"Cache-Control: no-store\r\n"+
		"Pragma: no-cache\r\n"+
		"X-Frame-Options: SAMEORIGIN\r\n"+
		"Strict-Transport-Security: max-age=31536000; includeSubDomains\r\n"+
		"X-Transcend-Version: 1\r\n"+
		"X-Aggregate-Auth: 1\r\n"+
		"%s"+
		"Connection: keep-alive\r\n\r\n%s",
		status, statusText, contentType, len(body), cookies, body)
	conn.Write([]byte(resp))
}
