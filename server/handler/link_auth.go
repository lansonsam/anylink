package handler

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"text/template"
	"time"

	"github.com/bjdgyc/anylink/base"
	"github.com/bjdgyc/anylink/dbdata"
	"github.com/bjdgyc/anylink/sessdata"
)

var (
	profileHash = ""
	certHash    = ""
)

func LinkAuth(w http.ResponseWriter, r *http.Request) {
	// TODO 调试信息输出
	if base.GetLogLevel() == base.LogLevelTrace {
		hd, _ := httputil.DumpRequest(r, true)
		base.Trace("LinkAuth: ", string(hd))
	}
	// 判断anyconnect客户端
	userAgent := strings.ToLower(r.UserAgent())
	xAggregateAuth := r.Header.Get("X-Aggregate-Auth")
	xTranscendVersion := r.Header.Get("X-Transcend-Version")
	if !((strings.Contains(userAgent, "anyconnect") || strings.Contains(userAgent, "openconnect") || strings.Contains(userAgent, "anylink")) &&
		xAggregateAuth == "1" && xTranscendVersion == "1") {
		w.WriteHeader(http.StatusForbidden)
		fmt.Fprintf(w, "error request")
		return
	}

	body, err := io.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	defer r.Body.Close()

	cr := &ClientRequest{
		RemoteAddr: r.RemoteAddr,
		UserAgent:  userAgent,
	}
	err = xml.Unmarshal(body, &cr)
	if err != nil {
		base.Error(err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	base.Trace(fmt.Sprintf("%+v \n", cr))
	// setCommonHeader(w)
	if cr.Type == "logout" {
		// 退出删除session信息
		if cr.SessionToken != "" {
			sessdata.DelSessByStoken(cr.SessionToken)
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	if cr.Type == "init" {
		w.WriteHeader(http.StatusOK)
		data := RequestData{Group: cr.GroupSelect, Groups: dbdata.GetGroupNamesNormal()}
		
		// 检查是否为OIDC认证组
		if cr.GroupSelect != "" {
			group := dbdata.GetGroup(cr.GroupSelect)
			base.Info("LinkAuth - Selected group:", cr.GroupSelect, "Group exists:", group != nil)
			if group != nil {
				base.Info("LinkAuth - Group Auth config:", group.Auth)
				if len(group.Auth) > 0 {
					if authType, ok := group.Auth["type"].(string); ok && authType == "oidc" {
						// 为OIDC认证设置SSO URL - 使用正版Cisco路由格式
						data.Host = r.Host
						data.SessionCtx = sessdata.GenerateSessionToken()[:8] // 生成唯一的会话上下文
						data.IsOidcAuth = true
						base.Info("LinkAuth - OIDC auth detected for group:", cr.GroupSelect)
						
						// 保存会话上下文信息
						ctxToken := &sessdata.TempTokenData{
							Token:     "ctx_" + data.SessionCtx,
							GroupName: cr.GroupSelect,
							CreatedAt: time.Now(),
							ExpiresAt: time.Now().Add(5 * time.Minute),
							Data: map[string]interface{}{
								"type": "session_context",
								"group": cr.GroupSelect,
							},
						}
						sessdata.StoreTempToken(ctxToken)
					} else {
						base.Info("LinkAuth - Non-OIDC auth type:", authType)
					}
				} else {
					base.Info("LinkAuth - Group has empty Auth config")
				}
			}
		}
		
		tplRequest(tpl_request, w, data)
		return
	}

	// 登陆参数判断
	if cr.Type != "auth-reply" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// 锁定状态判断
	if !lockManager.CheckLocked(cr.Auth.Username, r.RemoteAddr) {
		w.WriteHeader(http.StatusTooManyRequests)
		return
	}

	// 用户活动日志
	ua := &dbdata.UserActLog{
		Username:        cr.Auth.Username,
		GroupName:       cr.GroupSelect,
		RemoteAddr:      r.RemoteAddr,
		Status:          dbdata.UserAuthSuccess,
		DeviceType:      cr.DeviceId.DeviceType,
		PlatformVersion: cr.DeviceId.PlatformVersion,
	}

	sessionData := &AuthSession{
		ClientRequest: cr,
		UserActLog:    ua,
	}
	// TODO 用户密码校验
	ext := map[string]interface{}{"mac_addr": cr.MacAddressList.MacAddress}
	
	// 检查Cookie中的OIDC/VPN令牌
	tokenCookie, _ := r.Cookie("acSamlv2Token")
	var cookieToken string
	if tokenCookie != nil {
		cookieToken = tokenCookie.Value
	}
	
	// 检查是否为SSO令牌、VPN令牌或Cookie中有令牌
	if strings.HasPrefix(cr.Auth.Password, "oidc_token_") || strings.HasPrefix(cr.Auth.Password, "vpn_session_") || 
		cookieToken != "" || cr.Auth.SsoToken != "" || cr.Auth.Password == "sso-authenticated" {
		// 优先使用sso-token字段，其次密码字段，最后Cookie
		tokenToCheck := cr.Auth.SsoToken
		if tokenToCheck == "" {
			tokenToCheck = cr.Auth.Password
		}
		if tokenToCheck == "" || (!strings.HasPrefix(tokenToCheck, "oidc_token_") && !strings.HasPrefix(tokenToCheck, "vpn_session_")) {
			if cookieToken != "" {
				// 先尝试VPN会话令牌，如果不存在再尝试OIDC令牌
				vpnTokenToTry := "vpn_session_" + cookieToken
				if sessdata.GetTempToken(vpnTokenToTry) != nil {
					tokenToCheck = vpnTokenToTry
				} else {
					// Cookie中的令牌需要加上前缀才能被现有验证逻辑处理
					tokenToCheck = "oidc_token_" + cookieToken
				}
			}
		}
		
		base.Info("OIDC SSO token authentication attempt:", "token_length", len(tokenToCheck), "group", cr.GroupSelect, "remote_addr", r.RemoteAddr)
		
		// 处理OIDC SSO令牌
		username, groupName, err := handleOidcSsoToken(tokenToCheck, cr.GroupSelect)
		if err != nil {
			lockManager.UpdateLoginStatus(cr.Auth.Username, r.RemoteAddr, false)
			tokenPrefix := tokenToCheck
			if len(tokenPrefix) > 20 {
				tokenPrefix = tokenPrefix[:20] + "..."
			}
			base.Warn("OIDC SSO token validation failed:", err, "token_prefix", tokenPrefix, r.RemoteAddr)
			ua.Info = err.Error()
			ua.Status = dbdata.UserAuthFail
			dbdata.UserActLogIns.Add(*ua, userAgent)
			
			// 返回具体的认证错误给客户端
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`<?xml version="1.0" encoding="UTF-8"?>
<config-auth client="vpn" type="complete" aggregate-auth-version="2">
<error id="24" param1="" param2="">Authentication failed.</error>
</config-auth>`))
			return
		}
		
		base.Info("OIDC SSO token validation success:", "username", username, "group", groupName)
		// 设置从OIDC获取的用户名和组名
		cr.Auth.Username = username
		if cr.GroupSelect == "" && groupName != "" {
			cr.GroupSelect = groupName
			sessionData.ClientRequest.GroupSelect = groupName
			ua.GroupName = groupName
		}
		sessionData.ClientRequest.Auth.Username = username
		ua.Username = username
	} else {
		// 常规用户名密码认证
		err = dbdata.CheckUser(cr.Auth.Username, cr.Auth.Password, cr.GroupSelect, ext)
		if err != nil {
			// 检查是否是 OIDC 认证需要的特殊错误
			if err.Error() == "oidc:required" {
				// 获取组信息
				group := dbdata.GetGroup(cr.GroupSelect)
				if group != nil && len(group.Auth) > 0 {
					if authType, ok := group.Auth["type"].(string); ok && authType == "oidc" {
						// 返回 OIDC 登录 URL
						w.WriteHeader(http.StatusOK)
						oidcUrl := fmt.Sprintf("/oidc/login?group=%s", url.QueryEscape(cr.GroupSelect))
						data := &OidcRedirectData{
							OidcLoginUrl: oidcUrl,
						}
						tplRequest(tpl_oidc_redirect, w, data)
						return
					}
				}
			}
		}
	}
	
	if err != nil {
		
		lockManager.UpdateLoginStatus(cr.Auth.Username, r.RemoteAddr, false) // 记录登录失败状态

		base.Warn(err, r.RemoteAddr)
		ua.Info = err.Error()
		ua.Status = dbdata.UserAuthFail
		dbdata.UserActLogIns.Add(*ua, userAgent)

		w.WriteHeader(http.StatusOK)
		data := RequestData{Group: cr.GroupSelect, Groups: dbdata.GetGroupNamesNormal(), Error: "用户名或密码错误"}
		if base.Cfg.DisplayError {
			data.Error = err.Error()
		}
		tplRequest(tpl_request, w, data)
		return
	}
	dbdata.UserActLogIns.Add(*ua, userAgent)

	v := &dbdata.User{}
	err = dbdata.One("Username", cr.Auth.Username, v)
	if err != nil {
		base.Info("正在使用第三方认证方式登录")
		CreateSession(w, r, sessionData)
		return
	}
	// 用户otp验证
	if base.Cfg.AuthAloneOtp && !v.DisableOtp {
		lockManager.UpdateLoginStatus(cr.Auth.Username, r.RemoteAddr, true) // 重置OTP验证计数

		sessionID, err := GenerateSessionID()
		if err != nil {
			base.Error("Failed to generate session ID: ", err)
			http.Error(w, "Failed to generate session ID", http.StatusInternalServerError)
			return
		}

		sessionData.ClientRequest.Auth.OtpSecret = v.OtpSecret
		SessStore.SaveAuthSession(sessionID, sessionData)

		SetCookie(w, "auth-session-id", sessionID, 0)

		data := RequestData{}
		w.WriteHeader(http.StatusOK)
		tplRequest(tpl_otp, w, data)
		return
	}

	CreateSession(w, r, sessionData)
}

// handleOidcSsoToken 处理OIDC SSO令牌
func handleOidcSsoToken(token, groupName string) (string, string, error) {
	base.Info("Validating SSO token:", "token_exists", token != "", "group", groupName)
	
	// 处理不同格式的令牌
	var fullToken string
	var expectedType string
	
	if strings.HasPrefix(token, "oidc_token_") {
		// OIDC令牌
		fullToken = token
		expectedType = "oidc_session"
	} else if strings.HasPrefix(token, "vpn_session_") {
		// VPN会话令牌
		fullToken = token
		expectedType = "vpn_session"
	} else if token == "sso-authenticated" {
		// 特殊的SSO认证标识，需要从Cookie获取
		return "", "", fmt.Errorf("需要从Cookie获取令牌")
	} else {
		// 纯令牌，默认加上OIDC前缀
		fullToken = "oidc_token_" + token
		expectedType = "oidc_session"
	}
	
	// 验证令牌
	sessionData := sessdata.GetTempToken(fullToken)
	if sessionData == nil {
		base.Warn("Token not found in temporary storage:", "token_prefix", token[:min(len(token), 20)])
		return "", "", fmt.Errorf("无效的SSO令牌")
	}
	
	base.Info("Token found:", "type", sessionData.Data["type"], "group", sessionData.GroupName, "expires", sessionData.ExpiresAt)
	
	tokenType := sessionData.Data["type"].(string)
	if tokenType != expectedType {
		return "", "", fmt.Errorf("令牌类型错误: expected %s, got %s", expectedType, tokenType)
	}
	
	// 如果客户端没有指定组名，使用令牌中的组名
	if groupName == "" && sessionData.GroupName != "" {
		// 不需要检查，令牌中的组名是有效的
	} else if sessionData.GroupName != groupName {
		return "", "", fmt.Errorf("令牌组不匹配: expected %s, got %s", groupName, sessionData.GroupName)
	}
	
	// 获取用户名
	username, ok := sessionData.Data["username"].(string)
	if !ok || username == "" {
		return "", "", fmt.Errorf("令牌中缺少用户名")
	}
	
	base.Info("OIDC token validation successful:", "username", username)
	
	// 如果客户端没有指定组名，返回令牌中的组名
	actualGroupName := groupName
	if groupName == "" && sessionData.GroupName != "" {
		actualGroupName = sessionData.GroupName
	}
	
	// 令牌有效，删除临时令牌
	sessdata.DeleteTempToken(fullToken)
	return username, actualGroupName, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

const (
	tpl_request = iota
	tpl_complete
	tpl_otp
	tpl_oidc_redirect
)

func tplRequest(typ int, w io.Writer, data interface{}) {
	switch typ {
	case tpl_request:
		t, _ := template.New("auth_request").Parse(auth_request)
		_ = t.Execute(w, data)
	case tpl_complete:
		if rd, ok := data.(RequestData); ok && rd.Banner != "" {
			buf := new(bytes.Buffer)
			_ = xml.EscapeText(buf, []byte(rd.Banner))
			rd.Banner = buf.String()
			data = rd
		}
		t, _ := template.New("auth_complete").Parse(auth_complete)
		_ = t.Execute(w, data)
	case tpl_otp:
		t, _ := template.New("auth_otp").Parse(auth_otp)
		_ = t.Execute(w, data)
	case tpl_oidc_redirect:
		t, _ := template.New("oidc_redirect").Parse(oidc_redirect)
		_ = t.Execute(w, data)
	}
}

// 设置输出信息
type RequestData struct {
	Groups []string
	Group  string
	Error  string

	// complete
	SessionId    string
	SessionToken string
	Banner       string
	ProfileName  string
	ProfileHash  string
	CertHash     string
	
	// OIDC SSO
	OidcLoginUrl       string
	SsoFinalUrl        string
	SsoLogoutUrl       string
	SsoLogoutFinalUrl  string
	IsOidcAuth         bool
	Host               string
	SessionCtx         string
}

// OIDC 重定向数据
type OidcRedirectData struct {
	OidcLoginUrl string
}

var auth_request = `<?xml version="1.0" encoding="UTF-8"?>
<config-auth client="vpn" type="auth-request" aggregate-auth-version="2">
    <opaque is-for="sg">
        <tunnel-group>{{.Group}}</tunnel-group>
        <aggauth-handle>7260865041592658378</aggauth-handle>
        <auth-method>multiple-cert</auth-method>
        <auth-method>single-sign-on-v2</auth-method>
        <auth-method>single-sign-on-external-browser</auth-method>
        <config-hash>1739809980767</config-hash>
    </opaque>
    <auth id="main">
        {{if .IsOidcAuth}}
        <title>Login</title>
        <message>Please complete the authentication process in the AnyConnect Login window.</message>
        <banner>STOP: Before connecting you must be enrolled for Two-Step Verification. To learn more about Two-Step and how to enroll please visit https://nau.edu/Two-Step&#x0A;&#x0A;In the &#x22;Two-Step Verification&#x22; field, you will need to type the word &#x22;push&#x22; or the 6-digit verification code provided to you by either your Two-Step fob, or Duo Mobile phone app.</banner>
        <sso-v2-login>https://{{.Host}}/+CSCOE+/saml/sp/login?ctx={{.SessionCtx}}&#x26;acsamlcap=v2</sso-v2-login>
        <sso-v2-login-final>https://{{.Host}}/+CSCOE+/saml_ac_login.html</sso-v2-login-final>
        <sso-v2-token-cookie-name>acSamlv2Token</sso-v2-token-cookie-name>
        <sso-v2-error-cookie-name>acSamlv2Error</sso-v2-error-cookie-name>
        <form>
            <input type="sso" name="sso-token"></input>
        </form>
        {{else}}
        <title>Login</title>
        <message>请输入你的用户名和密码</message>
        <banner></banner>
        {{if .Error}}
        <error id="88" param1="{{.Error}}" param2="">登陆失败:  %s</error>
        {{end}}
        <form>
            <input type="text" name="username" label="Username:"></input>
            <input type="password" name="password" label="Password:"></input>
            <select name="group_list" label="GROUP:">
                {{range $v := .Groups}}
                <option {{if eq $v $.Group}} selected="true"{{end}}>{{$v}}</option>
                {{end}}
            </select>
        </form>
        {{end}}
    </auth>
</config-auth>
`

var auth_complete = `<?xml version="1.0" encoding="UTF-8"?>
<config-auth client="vpn" type="complete" aggregate-auth-version="2">
    <session-id>{{.SessionId}}</session-id>
    <session-token>{{.SessionToken}}</session-token>
    <auth id="success">
        <banner>{{.Banner}}</banner>
        <message id="0" param1="" param2=""></message>
    </auth>
    <capabilities>
        <crypto-supported>ssl-dhe</crypto-supported>
    </capabilities>
    <config client="vpn" type="private">
        <vpn-base-config>
            <server-cert-hash>{{.CertHash}}</server-cert-hash>
        </vpn-base-config>
        <opaque is-for="vpn-client"></opaque>
        <vpn-profile-manifest>
            <vpn rev="1.0">
                <file type="profile" service-type="user">
                    <uri>/profile_{{.ProfileName}}.xml</uri>
                    <hash type="sha1">{{.ProfileHash}}</hash>
                </file>
            </vpn>
        </vpn-profile-manifest>
    </config>
</config-auth>
`

// var auth_profile = `<?xml version="1.0" encoding="UTF-8"?>
// <AnyConnectProfile xmlns="http://schemas.xmlsoap.org/encoding/" xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance" xsi:schemaLocation="http://schemas.xmlsoap.org/encoding/ AnyConnectProfile.xsd">

// 	<ClientInitialization>
// 		<UseStartBeforeLogon UserControllable="false">false</UseStartBeforeLogon>
// 		<StrictCertificateTrust>false</StrictCertificateTrust>
// 		<RestrictPreferenceCaching>false</RestrictPreferenceCaching>
// 		<RestrictTunnelProtocols>IPSec</RestrictTunnelProtocols>
// 		<BypassDownloader>true</BypassDownloader>
// 		<WindowsVPNEstablishment>AllowRemoteUsers</WindowsVPNEstablishment>
// 		<CertEnrollmentPin>pinAllowed</CertEnrollmentPin>
// 		<CertificateMatch>
// 			<KeyUsage>
// 				<MatchKey>Digital_Signature</MatchKey>
// 			</KeyUsage>
// 			<ExtendedKeyUsage>
// 				<ExtendedMatchKey>ClientAuth</ExtendedMatchKey>
// 			</ExtendedKeyUsage>
// 		</CertificateMatch>

// 		<BackupServerList>
// 	            <HostAddress>localhost</HostAddress>
// 		</BackupServerList>
// 	</ClientInitialization>

//	<ServerList>
//		<HostEntry>
//	            <HostName>VPN Server</HostName>
//	            <HostAddress>localhost</HostAddress>
//		</HostEntry>
//	</ServerList>
//
// </AnyConnectProfile>
// `
var ds_domains_xml = `
<?xml version="1.0" encoding="UTF-8"?>
<config-auth client="vpn" type="complete" aggregate-auth-version="2">
    <config client="vpn" type="private">
        <opaque is-for="vpn-client">
            <custom-attr>
            {{if .DsExcludeDomains}}
               <dynamic-split-exclude-domains><![CDATA[{{.DsExcludeDomains}},]]></dynamic-split-exclude-domains>
            {{else if .DsIncludeDomains}}
               <dynamic-split-include-domains><![CDATA[{{.DsIncludeDomains}}]]></dynamic-split-include-domains>
            {{end}}
            </custom-attr>
        </opaque>
    </config>
</config-auth>
`

var oidc_redirect = `<?xml version="1.0" encoding="UTF-8"?>
<config-auth client="vpn" type="auth-request" aggregate-auth-version="2">
    <opaque is-for="sg">
        <auth-method>single-sign-on-v2</auth-method>
    </opaque>
    <auth id="main">
        <title>OIDC Authentication Required</title>
        <message>Please complete authentication in your browser</message>
        <sso>
            <sso-url>{{.OidcLoginUrl}}</sso-url>
        </sso>
    </auth>
</config-auth>
`
