package tokens

import (
	"errors"
	"time"

	"strings"
	"yunion.io/x/onecloud/pkg/keystone/keys"
	"yunion.io/x/onecloud/pkg/keystone/models"
	"yunion.io/x/onecloud/pkg/keystone/options"
	"yunion.io/x/onecloud/pkg/mcclient"
)

var (
	ErrInvalidFernetToken = errors.New("invalid fernet token")
)

type SAuthToken struct {
	UserId    string
	Method    string
	ProjectId string
	DomainId  string
	ExpiresAt time.Time
	AuditIds  []string

	// Token string
}

func (t *SAuthToken) Decode(tk []byte) error {
	for _, payload := range []ITokenPayload{
		&SProjectScopedPayload{},
		&SDomainScopedPayload{},
		&SUnscopedPayload{},
	} {
		err := payload.Unmarshal(tk)
		if err == nil {
			payload.Decode(t)
			return nil
		}
	}
	return ErrInvalidFernetToken
}

func (t *SAuthToken) getProjectScopedPayload() ITokenPayload {
	p := SProjectScopedPayload{}
	p.Version = SProjectScopedPayloadVersion
	p.UserId.parse(t.UserId)
	p.ProjectId.parse(t.ProjectId)
	p.Method = authMethodStr2Id(t.Method)
	p.ExpiresAt = float64(t.ExpiresAt.Unix())
	p.AuditIds = auditStrings2Bytes(t.AuditIds)
	return &p
}

func (t *SAuthToken) getDomainScopedPayload() ITokenPayload {
	p := SDomainScopedPayload{}
	p.Version = SDomainScopedPayloadVersion
	p.UserId.parse(t.UserId)
	p.DomainId.parse(t.DomainId)
	p.Method = authMethodStr2Id(t.Method)
	p.ExpiresAt = float64(t.ExpiresAt.Unix())
	p.AuditIds = auditStrings2Bytes(t.AuditIds)
	return &p
}

func (t *SAuthToken) getUnscopedPayload() ITokenPayload {
	p := SUnscopedPayload{}
	p.Version = SUnscopedPayloadVersion
	p.UserId.parse(t.UserId)
	p.Method = authMethodStr2Id(t.Method)
	p.ExpiresAt = float64(t.ExpiresAt.Unix())
	p.AuditIds = auditStrings2Bytes(t.AuditIds)
	return &p
}

func (t *SAuthToken) getPayload() ITokenPayload {
	if len(t.ProjectId) > 0 {
		return t.getProjectScopedPayload()
	}
	if len(t.DomainId) > 0 {
		return t.getDomainScopedPayload()
	}
	return t.getUnscopedPayload()
}

func (t *SAuthToken) Encode() ([]byte, error) {
	return t.getPayload().Encode()
}

func (t *SAuthToken) ParseFernetToken(tokenStr string) error {
	tk := keys.TokenKeysManager.Decrypt([]byte(tokenStr), time.Duration(options.Options.TokenExpirationSeconds)*time.Second)
	err := t.Decode(tk)
	if err != nil {
		return err
	}
	return nil
}

func (t *SAuthToken) EncodeFernetToken() (string, error) {
	tk, err := t.Encode()
	if err != nil {
		return "", err
	}
	ftk, err := keys.TokenKeysManager.Encrypt(tk)
	if err != nil {
		return "", err
	}
	return string(ftk), nil
}

func (t *SAuthToken) GetSimpleUserCred(token string) (mcclient.TokenCredential, error) {
	userExt, err := models.UserManager.FetchUserExtended(t.UserId, "", "", "")
	if err != nil {
		return nil, err
	}
	ret := mcclient.SSimpleToken{
		Token:   token,
		UserId:  t.UserId,
		User:    userExt.Name,
		Expires: t.ExpiresAt,
	}
	var roles []models.SRole
	if len(t.ProjectId) > 0 {
		proj, err := models.ProjectManager.FetchProjectById(t.ProjectId)
		if err != nil {
			return nil, err
		}
		ret.ProjectId = t.ProjectId
		ret.Project = proj.Name
		roles, err = models.AssignmentManager.FetchUserProjectRoles(t.UserId, t.ProjectId)
	} else if len(t.DomainId) > 0 {
		domain, err := models.DomainManager.FetchDomainById(t.DomainId)
		if err != nil {
			return nil, err
		}

		ret.DomainId = t.DomainId
		ret.Domain = domain.Name
		roles, err = models.AssignmentManager.FetchUserProjectRoles(t.UserId, t.DomainId)
	}
	roleStrs := make([]string, len(roles))
	for i := range roles {
		roleStrs[i] = roles[i].Name
	}
	ret.Roles = strings.Join(roleStrs, ",")
	return &ret, nil
}

func (t *SAuthToken) getRoles() ([]models.SRole, error) {
	var roleProjectId string
	if len(t.ProjectId) > 0 {
		roleProjectId = t.ProjectId
	} else if len(t.DomainId) > 0 {
		roleProjectId = t.DomainId
	}
	if len(roleProjectId) > 0 {
		return models.AssignmentManager.FetchUserProjectRoles(t.UserId, roleProjectId)
	}
	return nil, nil
}

func (t *SAuthToken) getTokenV3(
	user *models.SUserExtended,
	project *models.SProjectExtended,
	domain *models.SDomain,
) (*mcclient.TokenCredentialV3, error) {
	token := mcclient.TokenCredentialV3{}
	token.Token.ExpiresAt = t.ExpiresAt
	token.Token.IssuedAt = t.ExpiresAt.Add(-time.Duration(options.Options.TokenExpirationSeconds) * time.Second)
	token.Token.AuditIds = t.AuditIds
	token.Token.Methods = []string{t.Method}
	token.Token.User.Id = user.Id
	token.Token.User.Name = user.Name
	token.Token.User.Domain.Id = user.DomainId
	token.Token.User.Domain.Name = user.DomainName

	tk, err := t.EncodeFernetToken()
	if err != nil {
		return nil, err
	}
	token.Id = tk

	roles, err := t.getRoles()
	if err != nil {
		return nil, err
	}

	if len(roles) > 0 {
		if project != nil {
			token.Token.IsDomain = false
			token.Token.Project.Id = project.Id
			token.Token.Project.Name = project.Name
			token.Token.Project.Domain.Id = project.DomainId
			token.Token.Project.Domain.Name = project.DomainName
		} else if domain != nil {
			token.Token.IsDomain = true
			token.Token.Project.Id = domain.Id
			token.Token.Project.Name = domain.Name
		}

		token.Token.Roles = make([]mcclient.KeystoneRoleV3, len(roles))
		for i := range roles {
			token.Token.Roles[i].Id = roles[i].Id
			token.Token.Roles[i].Name = roles[i].Name
		}

		endpoints, err := models.EndpointManager.FetchAll()
		if err != nil {
			return nil, err
		}
		if endpoints != nil {
			token.Token.Catalog = endpoints.GetKeystoneCatalogV3()
		}
	}

	return &token, nil
}

func (t *SAuthToken) getTokenV2(
	user *models.SUserExtended,
	project *models.SProject,
) (*mcclient.TokenCredentialV2, error) {
	token := mcclient.TokenCredentialV2{}
	token.User.Name = user.Name
	token.User.Id = user.Id
	token.User.Username = user.Name

	tk, err := t.EncodeFernetToken()
	if err != nil {
		return nil, err
	}
	token.Token.Id = tk
	token.Token.Expires = t.ExpiresAt

	roles, err := t.getRoles()
	if err != nil {
		return nil, err
	}

	if len(roles) > 0 {
		token.Token.Tenant.Id = project.Id
		token.Token.Tenant.Name = project.Name

		token.User.Roles = make([]mcclient.KeystoneRoleV2, len(roles))
		token.Metadata.Roles = make([]string, len(roles))
		for i := range roles {
			token.User.Roles[i].Name = roles[i].Name
			token.Metadata.Roles[i] = roles[i].Name
		}
		endpoints, err := models.EndpointManager.FetchAll()
		if err != nil {
			return nil, err
		}
		if endpoints != nil {
			token.ServiceCatalog = endpoints.GetKeystoneCatalogV2()
		}
	}

	return &token, nil
}
