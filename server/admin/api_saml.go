package admin

import (
	"encoding/json"
	"net/http"

	"github.com/bjdgyc/anylink/dbdata"
)

// SAML 配置示例
type SamlConfigExample struct {
	Type             string `json:"type"`
	IdpMetadataUrl   string `json:"idp_metadata_url"`
	IdpSsoUrl        string `json:"idp_sso_url"`
	IdpEntityId      string `json:"idp_entity_id"`
	SpEntityId       string `json:"sp_entity_id"`
	SpAcsUrl         string `json:"sp_acs_url"`
	IdpCertificate   string `json:"idp_certificate"`
	AttributeMapping struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Groups   string `json:"groups"`
	} `json:"attribute_mapping"`
}

// 获取 SAML 配置示例
func SamlExample(w http.ResponseWriter, r *http.Request) {
	// 获取当前服务器地址
	scheme := "https"
	if r.TLS == nil {
		scheme = "http"
	}
	host := r.Host
	
	example := SamlConfigExample{
		Type:           "saml",
		IdpMetadataUrl: "https://your-idp.com/metadata.xml",
		IdpSsoUrl:      "https://your-idp.com/sso/saml",
		IdpEntityId:    "https://your-idp.com/entity",
		SpEntityId:     scheme + "://" + host,
		SpAcsUrl:       scheme + "://" + host + "/saml/acs",
		IdpCertificate: "-----BEGIN CERTIFICATE-----\n...\n-----END CERTIFICATE-----",
		AttributeMapping: struct {
			Username string `json:"username"`
			Email    string `json:"email"`
			Groups   string `json:"groups"`
		}{
			Username: "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/name",
			Email:    "http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
			Groups:   "http://schemas.microsoft.com/ws/2008/06/identity/claims/groups",
		},
	}
	
	RespSucess(w, example)
}

// 测试 SAML 配置
func TestSamlConfig(w http.ResponseWriter, r *http.Request) {
	body := &struct {
		GroupName string                 `json:"group_name"`
		Auth      map[string]interface{} `json:"auth"`
	}{}
	
	err := json.NewDecoder(r.Body).Decode(body)
	if err != nil {
		RespError(w, RespInternalErr, err.Error())
		return
	}
	
	// 验证组是否存在
	group := dbdata.GetGroup(body.GroupName)
	if group == nil {
		RespError(w, RespParamErr, "组不存在")
		return
	}
	
	// 测试认证配置
	auth, err := dbdata.GetAuth(body.GroupName)
	if err != nil {
		RespError(w, RespInternalErr, err.Error())
		return
	}
	
	if samlAuth, ok := auth.(*dbdata.AuthSaml); ok {
		// 生成测试登录 URL
		_, redirectUrl := samlAuth.GetSamlRequest(body.GroupName)
		result := map[string]string{
			"test_login_url": redirectUrl,
			"message":        "SAML 配置有效，请访问测试登录 URL",
		}
		RespSucess(w, result)
	} else {
		RespError(w, RespParamErr, "不是 SAML 认证类型")
	}
}