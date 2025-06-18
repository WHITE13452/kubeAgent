package controller

import (
	"ginK8s/models"
	"ginK8s/pkg/services"

	"github.com/gin-gonic/gin"
)

type ResourceController struct {
	ResourceService *services.ResourceService // 资源服务
}

func NewResourceController(resourceService *services.ResourceService) *ResourceController {
	return &ResourceController{
		ResourceService: resourceService,
	}
}

func (r *ResourceController) GetResourceList(c *gin.Context) {
	resourceName := c.Param("resourceName")
	if resourceName == "" {
		c.JSON(400, gin.H{"error": "resourceName is required"})
		return
	}
	ns := c.DefaultQuery("namespace", "default")
	resources, err := r.ResourceService.GetResourceList(resourceName, ns)
	if err != nil {	
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, resources)
}

func (r *ResourceController) CreateResource(c *gin.Context) {
	resourceName := c.Param("resourcename")
	if resourceName == "" {
		c.JSON(400, gin.H{"error": "resourceName is required"})
		return
	}
	var param models.ResourceParam
	if err := c.ShouldBindBodyWithJSON(&param); err != nil {
		c.JSON(400, gin.H{"error": "Invalid request body"})
		return
	}

	err := r.ResourceService.CreateResource(resourceName, param.Yaml)
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}

	c.JSON(200, gin.H{"message": "Resource created successfully"})
}

func (r *ResourceController) DeleteResource(c *gin.Context) {
	resourceName := c.Param("resourceName")
	if resourceName == "" {
		c.JSON(400, gin.H{"error": "resourceName is required"})
		return
	}
	// ns := c.DefaultQuery("namespace", "default")

	// err := r.ResourceService.DeleteResource(resourceName, ns)
	// if err != nil {
	// 	c.JSON(500, gin.H{"error": err.Error()})
	// 	return
	// }

	c.JSON(200, gin.H{"message": "Resource deleted successfully"})
}