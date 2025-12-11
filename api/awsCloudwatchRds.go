package api

import (
	"github.com/gin-gonic/gin"
	"alertHub/internal/middleware"
	"alertHub/internal/services"
	"alertHub/pkg/community/aws/cloudwatch/types"
)

type awsCloudWatchRDSController struct{}

var AWSCloudWatchRDSController = new(awsCloudWatchRDSController)

func (awsCloudWatchRDSController awsCloudWatchRDSController) API(gin *gin.RouterGroup) {
	community := gin.Group("community")
	community.Use(
		middleware.Cors(),
		middleware.Auth(),
		middleware.ParseTenant(),
	)
	{
		rds := community.Group("rds")
		{
			rds.GET("instances", awsCloudWatchRDSController.GetRdsInstanceIdentifier)
			rds.GET("clusters", awsCloudWatchRDSController.GetRdsClusterIdentifier)
		}
	}
}

func (awsCloudWatchRDSController awsCloudWatchRDSController) GetRdsInstanceIdentifier(ctx *gin.Context) {
	req := new(types.RdsInstanceReq)
	BindQuery(ctx, req)
	Service(ctx, func() (interface{}, interface{}) {
		return services.AWSCloudWatchRdsService.GetDBInstanceIdentifier(req)
	})
}

func (awsCloudWatchRDSController awsCloudWatchRDSController) GetRdsClusterIdentifier(ctx *gin.Context) {
	req := new(types.RdsClusterReq)
	BindQuery(ctx, req)
	Service(ctx, func() (interface{}, interface{}) {
		return services.AWSCloudWatchRdsService.GetDBClusterIdentifier(req)
	})
}
