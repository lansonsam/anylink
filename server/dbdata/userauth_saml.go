package dbdata

import (
	"encoding/base64"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"net/url"
	"reflect"
	"time"

	"github.com/google/uuid"
)

type AuthSaml struct {
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

type SamlSession struct {
	Id        string
	GroupName string
	RequestId string
	CreatedAt time.Time
}

var samlSessions = make(map[string]*SamlSession)

func init() {
	authRegistry["saml"] = reflect.TypeOf(AuthSaml{})
}

func (s *AuthSaml) checkData(authData map[string]interface{}) error {
	// 验证必须的字段
	if s.IdpSsoUrl == "" {
		return fmt.Errorf("IdP SSO URL is required")
	}
	if s.SpEntityId == "" {
		return fmt.Errorf("SP Entity ID is required")
	}
	if s.SpAcsUrl == "" {
		return fmt.Errorf("SP ACS URL is required")
	}
	return nil
}

func (s *AuthSaml) checkUser(name, pwd string, g *Group, ext map[string]interface{}) error {
	// SAML 认证不使用用户名密码，直接返回特殊错误让客户端打开浏览器
	return fmt.Errorf("saml:required")
}

func (s *AuthSaml) GetSamlRequest(group string) (string, string) {
	requestId := uuid.New().String()
	sessionId := uuid.New().String()
	
	// 保存会话信息
	samlSessions[sessionId] = &SamlSession{
		Id:        sessionId,
		GroupName: group,
		RequestId: requestId,
		CreatedAt: time.Now(),
	}
	
	// 清理过期会话（超过30分钟）
	for k, v := range samlSessions {
		if time.Since(v.CreatedAt) > 30*time.Minute {
			delete(samlSessions, k)
		}
	}
	
	// 构建 SAML AuthnRequest
	authnRequest := `<samlp:AuthnRequest xmlns:samlp="urn:oasis:names:tc:SAML:2.0:protocol"
                    xmlns:saml="urn:oasis:names:tc:SAML:2.0:assertion"
                    ID="` + requestId + `"
                    Version="2.0"
                    IssueInstant="` + time.Now().UTC().Format(time.RFC3339) + `"
                    Destination="` + s.IdpSsoUrl + `"
                    AssertionConsumerServiceURL="` + s.SpAcsUrl + `"
                    ProtocolBinding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-POST">
    <saml:Issuer>` + s.SpEntityId + `</saml:Issuer>
    <samlp:NameIDPolicy Format="urn:oasis:names:tc:SAML:1.1:nameid-format:unspecified"
                        AllowCreate="true" />
</samlp:AuthnRequest>`
	
	// Base64 编码
	encoded := base64.StdEncoding.EncodeToString([]byte(authnRequest))
	
	// 构建重定向 URL
	redirectUrl := fmt.Sprintf("%s?SAMLRequest=%s&RelayState=%s",
		s.IdpSsoUrl,
		url.QueryEscape(encoded),
		sessionId)
	
	return sessionId, redirectUrl
}

type SamlResponse struct {
	XMLName   xml.Name `xml:"urn:oasis:names:tc:SAML:2.0:protocol Response"`
	Status    struct {
		StatusCode struct {
			Value string `xml:",attr"`
		} `xml:"StatusCode"`
	} `xml:"Status"`
	Assertion struct {
		Subject struct {
			NameID struct {
				Value string `xml:",chardata"`
			} `xml:"NameID"`
		} `xml:"Subject"`
		AttributeStatement struct {
			Attributes []struct {
				Name       string `xml:",attr"`
				NameFormat string `xml:",attr"`
				Values     []struct {
					Value string `xml:",chardata"`
				} `xml:"AttributeValue"`
			} `xml:"Attribute"`
		} `xml:"AttributeStatement"`
	} `xml:"Assertion"`
}

func (s *AuthSaml) ValidateSamlResponse(responseData string) (string, map[string]string, error) {
	// Base64 解码
	decoded, err := base64.StdEncoding.DecodeString(responseData)
	if err != nil {
		return "", nil, fmt.Errorf("failed to decode SAML response: %v", err)
	}
	
	// 解析 XML
	var response SamlResponse
	err = xml.Unmarshal(decoded, &response)
	if err != nil {
		return "", nil, fmt.Errorf("failed to parse SAML response: %v", err)
	}
	
	// 检查状态
	if response.Status.StatusCode.Value != "urn:oasis:names:tc:SAML:2.0:status:Success" {
		return "", nil, fmt.Errorf("SAML authentication failed")
	}
	
	// TODO: 验证签名（需要解析 IdP 证书）
	
	// 提取属性
	attributes := make(map[string]string)
	username := response.Assertion.Subject.NameID.Value
	
	for _, attr := range response.Assertion.AttributeStatement.Attributes {
		if len(attr.Values) > 0 {
			attributes[attr.Name] = attr.Values[0].Value
		}
	}
	
	// 根据配置映射用户名
	if s.AttributeMapping.Username != "" {
		if val, ok := attributes[s.AttributeMapping.Username]; ok {
			username = val
		}
	}
	
	return username, attributes, nil
}

func GetSamlSession(sessionId string) (*SamlSession, bool) {
	session, exists := samlSessions[sessionId]
	return session, exists
}

func DeleteSamlSession(sessionId string) {
	delete(samlSessions, sessionId)
}

// GetAuth 获取认证实例
func GetAuth(groupName string) (interface{}, error) {
	group := GetGroup(groupName)
	if group == nil {
		return nil, fmt.Errorf("group not found")
	}
	
	if len(group.Auth) == 0 {
		return nil, fmt.Errorf("no auth config")
	}
	
	authType, ok := group.Auth["type"].(string)
	if !ok {
		return nil, fmt.Errorf("invalid auth type")
	}
	
	auth := makeInstance(authType)
	
	// 将 map 转换为 JSON，再解析到结构体
	authJson, err := json.Marshal(group.Auth)
	if err != nil {
		return nil, err
	}
	
	err = json.Unmarshal(authJson, auth)
	if err != nil {
		return nil, err
	}
	
	return auth, nil
}