package api

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/SkyPanel/SkyPanel/v3/internal/middleware"
	"github.com/SkyPanel/SkyPanel/v3/internal/models"
	"github.com/SkyPanel/SkyPanel/v3/internal/response"
	"github.com/SkyPanel/SkyPanel/v3/internal/scopes"
	"github.com/SkyPanel/SkyPanel/v3/internal/services"
	"github.com/SkyPanel/SkyPanel/v3/pkg/skypanel"
	"github.com/gin-gonic/gin"
	"github.com/gin-gonic/gin/binding"
	"github.com/spf13/cast"
	"gorm.io/gorm"
)

func registerTemplates(g *gin.RouterGroup) {
	// GET endpoints only require login — any authenticated user needs to read templates
	// (e.g. for the server splitter). The page /templates is gated on the frontend via templates.view.
	g.Handle("GET", "", middleware.RequiresPermission(scopes.ScopeLogin), getRepos)
	g.Handle("POST", "", middleware.RequiresPermission(scopes.ScopeTemplatesRepoCreate), addRepo)
	g.Handle("OPTIONS", "", response.CreateOptions("GET", "POST"))

	g.Handle("POST", "/upload", middleware.RequiresPermission(scopes.ScopeTemplatesLocalEdit), uploadTemplates)
	g.Handle("OPTIONS", "/upload", response.CreateOptions("POST"))

	g.Handle("GET", "/:repo", middleware.RequiresPermission(scopes.ScopeLogin), getsTemplatesForRepo)
	g.Handle("DELETE", "/:repo", middleware.RequiresPermission(scopes.ScopeTemplatesRepoDelete), deleteRepo)
	g.Handle("OPTIONS", "/:repo", response.CreateOptions("GET", "PUT", "DELETE"))

	g.Handle("GET", "/:repo/:name", middleware.RequiresPermission(scopes.ScopeLogin), getTemplateFromRepo)
	g.Handle("DELETE", "/0/:name", middleware.RequiresPermission(scopes.ScopeTemplatesLocalEdit), deleteTemplate)
	g.Handle("PUT", "/0/:name", middleware.RequiresPermission(scopes.ScopeTemplatesLocalEdit), putTemplate)
	g.Handle("OPTIONS", "/:repo/:name", response.CreateOptions("GET"))
	g.Handle("OPTIONS", "/0/:name", response.CreateOptions("GET", "DELETE", "PUT"))
}

// @Summary Get all repos
// @Description Gets all repos that are available to pull template from
// @Success 200 {object} []models.TemplateRepo
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Tags Templates
// @Router /api/templates [get]
// @Security OAuth2Application[templates.view]
func getRepos(c *gin.Context) {
	db := middleware.GetDatabase(c)
	ts := &services.Template{DB: db}

	repos, err := ts.GetRepos()
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, repos)
}

// @Summary Get all templates from repo
// @Description Gets all templates from a repository
// @Param repo path uint true "Repo id"
// @Success 200 {object} []models.Template
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Tags Templates
// @Router /api/templates/{repo} [get]
// @Security OAuth2Application[templates.view]
func getsTemplatesForRepo(c *gin.Context) {
	db := middleware.GetDatabase(c)
	ts := &services.Template{DB: db}

	repoID, err := cast.ToUintE(c.Param("repo"))
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	templates, err := ts.GetAllFromRepo(repoID)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, templates)
}

// @Summary Add repo
// @Description Adds a new repo to the service
// @Param repo body templateRepoInput true "Repo information; pat, username, password and sshKey are optional and never returned"
// @Success 200 {object} models.TemplateRepo
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Tags Templates
// @Router /api/templates [post]
// @Security OAuth2Application[templates.repo.create]
func addRepo(c *gin.Context) {
	var input templateRepoInput
	err := c.BindJSON(&input)

	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	repo := input.toRepo()

	if repo.Name == "" {
		response.HandleError(c, skypanel.ErrFieldRequired("repoName"), http.StatusBadRequest)
		return
	}

	db := middleware.GetDatabase(c)
	ts := &services.Template{DB: db}

	err = ts.AddRepo(repo)

	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.JSON(http.StatusOK, repo)
}

// @Summary Delete repo
// @Description Deletes a repo from the service. Refused while a store product finds its template only in this repo.
// @Param repo path uint true "Repo Id"
// @Success 204 {object} nil
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 409 {object} skypanel.ErrorResponse "ErrTemplateRepoInUse: products use templates only this repo has"
// @Failure 500 {object} skypanel.ErrorResponse
// @Tags Templates
// @Router /api/templates/{repo} [delete]
// @Security OAuth2Application[templates.repo.delete]
func deleteRepo(c *gin.Context) {
	repoID, err := cast.ToUintE(c.Param("repo"))
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	db := middleware.GetDatabase(c)
	ts := &services.Template{DB: db}

	err = ts.DeleteRepo(repoID)
	if errors.Is(err, skypanel.ErrTemplateRepoInUse(nil)) {
		response.HandleError(c, err, http.StatusConflict)
		return
	} else if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}
	c.Status(http.StatusNoContent)
}

// @Summary Get template
// @Description Gets a template from the repo
// @Param repo path uint true "Repo Id"
// @Param template path string true "Template name"
// @Success 200 {object} models.Template
// @Failure 500 {object} skypanel.ErrorResponse
// @Tags Templates
// @Router /api/templates/{repo}/{template} [get]
// @Security OAuth2Application[templates.view]
func getTemplateFromRepo(c *gin.Context) {
	db := middleware.GetDatabase(c)
	ts := &services.Template{DB: db}

	repoID, err := cast.ToUintE(c.Param("repo"))
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	template, err := ts.Get(repoID, c.Param("name"))
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.AbortWithStatus(404)
		return
	} else if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.JSON(http.StatusOK, template)
}

// @Summary Adds or updates a template
// @Success 204 {object} nil
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 413 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Param template body skypanel.Server true "Template"
// @Param name path string true "Template name"
// @Tags Templates
// @Router /api/templates/local/{name} [put]
// @Security OAuth2Application[templates.local.edit]
func putTemplate(c *gin.Context) {
	db := middleware.GetDatabase(c)
	ts := &services.Template{DB: db}

	templateName := c.Param("name")

	// the body is stored whole, unknown keys included, so it gets the same cap as an upload
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxTemplateFileSize)
	// the body is checked as a server definition, as always, and then stored as written:
	// decoding it into skypanel.Server would sort the variables and drop unknown fields
	body, err := io.ReadAll(c.Request.Body)
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		response.HandleError(c, skypanel.CreateError("the template is larger than 2 MB", "ErrTemplateTooLarge"), http.StatusRequestEntityTooLarge)
		return
	}
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	c.Request.Body = io.NopCloser(bytes.NewReader(body))
	templateRequest := skypanel.Server{}
	err = c.MustBindWith(&templateRequest, binding.JSON)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	template, err := ts.Get(ts.GetLocalRepoID(), templateName)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		template = &models.Template{
			Name: templateName,
		}
	} else if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	// only a JSON null gets this far without being an object
	err = template.SetDefinition(body)
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}
	err = ts.Save(template)
	if response.HandleError(c, err, http.StatusInternalServerError) {
		return
	}

	c.Status(http.StatusNoContent)
}

// @Summary Deletes template
// @Description Deletes template. Refused while a store product creates its servers with it and no other repo has a template of that name.
// @Success 204 {object} nil
// @Failure 404 {object} skypanel.ErrorResponse
// @Failure 409 {object} skypanel.ErrorResponse "ErrTemplateInUse: products use this template"
// @Failure 500 {object} skypanel.ErrorResponse
// @Param name path string true "Template name"
// @Tags Templates
// @Router /api/templates/local/{name} [delete]
// @Security OAuth2Application[templates.repo.delete]
func deleteTemplate(c *gin.Context) {
	db := middleware.GetDatabase(c)
	ts := &services.Template{DB: db}

	err := ts.Delete(c.Param("name"))
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		c.AbortWithStatus(http.StatusNotFound)
		return
	case errors.Is(err, skypanel.ErrTemplateInUse(nil)):
		response.HandleError(c, err, http.StatusConflict)
		return
	case response.HandleError(c, err, http.StatusInternalServerError):
		return
	}

	c.Status(http.StatusNoContent)
}

const maxTemplateFileSize = 2 << 20 // 2 MB per template file

// @Summary Upload local templates
// @Description Uploads one or more .json template files to the local templates repository.
// @Description The template name is taken from the file name (e.g. minecraft.json -> "minecraft").
// @Accept multipart/form-data
// @Param files formData file true "Template JSON file(s)"
// @Success 200 {object} []models.Template
// @Failure 400 {object} skypanel.ErrorResponse
// @Failure 500 {object} skypanel.ErrorResponse
// @Tags Templates
// @Router /api/templates/upload [post]
// @Security OAuth2Application[templates.local.edit]
func uploadTemplates(c *gin.Context) {
	db := middleware.GetDatabase(c)
	ts := &services.Template{DB: db}

	form, err := c.MultipartForm()
	if response.HandleError(c, err, http.StatusBadRequest) {
		return
	}

	files := form.File["files"]
	if len(files) == 0 {
		response.HandleError(c, skypanel.ErrFieldRequired("files"), http.StatusBadRequest)
		return
	}

	created := make([]*models.Template, 0, len(files))
	for _, fh := range files {
		template, err := parseUploadedTemplate(fh)
		if response.HandleError(c, err, http.StatusBadRequest) {
			return
		}

		if err := ts.Save(template); response.HandleError(c, err, http.StatusInternalServerError) {
			return
		}
		created = append(created, template)
	}

	c.JSON(http.StatusOK, created)
}

// parseUploadedTemplate reads one uploaded .json file into a template named
// after the file.
func parseUploadedTemplate(fh *multipart.FileHeader) (*models.Template, error) {
	if !strings.HasSuffix(strings.ToLower(fh.Filename), ".json") {
		return nil, skypanel.CreateError("file "+fh.Filename+" is not a JSON template", "ErrInvalidTemplate")
	}

	f, err := fh.Open()
	if err != nil {
		return nil, err
	}
	data, err := io.ReadAll(io.LimitReader(f, maxTemplateFileSize))
	_ = f.Close()
	if err != nil {
		return nil, err
	}

	var server skypanel.Server
	if err := json.Unmarshal(data, &server); err != nil {
		return nil, err
	}

	if len(server.Installation) == 0 && server.Execution.Command == nil {
		return nil, skypanel.CreateError("template "+fh.Filename+" has no install steps or run command", "ErrInvalidTemplate")
	}

	name := strings.TrimSuffix(filepath.Base(fh.Filename), filepath.Ext(fh.Filename))
	if name == "" {
		name = server.Type.Type
	}

	// the file is kept as written; a missing display name is added at its end
	template := &models.Template{Name: name}
	if err := template.SetDefinition(data); err != nil {
		return nil, err
	}
	if template.Display == "" {
		if err := template.SetField("display", name); err != nil {
			return nil, err
		}
	}
	return template, nil
}

// templateRepoInput is the body of POST /api/templates. The credentials of a private
// repository are accepted here but never sent back: models.TemplateRepo hides them from JSON.
type templateRepoInput struct {
	models.TemplateRepo
	PAT      string `json:"pat"`
	Username string `json:"username"`
	Password string `json:"password"`
	SSHKey   string `json:"sshKey"`
} // @name TemplateRepoInput

func (in templateRepoInput) toRepo() *models.TemplateRepo {
	repo := in.TemplateRepo
	repo.PAT = in.PAT
	repo.Username = in.Username
	repo.Password = in.Password
	repo.SSHKey = in.SSHKey
	return &repo
}
