/*
Copyright 2021 The Pixiu Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package options

import (
	"context"
	"fmt"
	"os"

	"github.com/bndr/gojenkins"
	pixiuConfig "github.com/caoyingjunz/pixiulib/config"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"github.com/caoyingjunz/gopixiu/cmd/app/config"
	"github.com/caoyingjunz/gopixiu/pkg/db"
	"github.com/caoyingjunz/gopixiu/pkg/log"
	"github.com/caoyingjunz/gopixiu/pkg/types"
)

const (
	maxIdleConns = 10
	maxOpenConns = 100

	defaultConfigFile = "/etc/gopixiu/config.yaml"
)

// Options has all the params needed to run a pixiu
type Options struct {
	// The default values.
	ComponentConfig config.Config
	GinEngine       *gin.Engine

	DB      *gorm.DB
	Factory db.ShareDaoFactory // 数据库接口

	// CICD 的驱动接口
	CicdDriver *gojenkins.Jenkins

	// ConfigFile is the location of the pixiu server's configuration file.
	ConfigFile string
}

func NewOptions() (*Options, error) {
	return &Options{}, nil
}

func (o *Options) Complete() error {
	configFile := o.ConfigFile
	if len(configFile) == 0 {
		configFile = os.Getenv("ConfigFile")
	}
	if len(configFile) == 0 {
		configFile = defaultConfigFile
	}

	c := pixiuConfig.New()
	c.SetConfigFile(configFile)
	c.SetConfigType("yaml")
	if err := c.Binding(&o.ComponentConfig); err != nil {
		return err
	}

	// 初始化默认 api 路由
	o.GinEngine = gin.Default()

	// 注册依赖组件
	if err := o.register(); err != nil {
		return err
	}
	return nil
}

func (o *Options) register() error {
	log.Register(o.ComponentConfig.Default.LogDir, o.ComponentConfig.Default.LogLevel) // 注册日志
	//if err := o.registerDatabase(); err != nil {                                        // 注册数据库
	//	return err
	//}
	if err := o.registerCicdDriver(); err != nil { // 注册 CICD driver
		return err
	}

	return nil
}

func (o *Options) registerDatabase() error {
	sqlConfig := o.ComponentConfig.Mysql
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8&parseTime=True&loc=Local",
		sqlConfig.User,
		sqlConfig.Password,
		sqlConfig.Host,
		sqlConfig.Port,
		sqlConfig.Name)

	var err error
	if o.DB, err = gorm.Open(mysql.Open(dsn), &gorm.Config{}); err != nil {
		return err
	}
	// 设置数据库连接池
	sqlDB, err := o.DB.DB()
	if err != nil {
		return err
	}
	sqlDB.SetMaxIdleConns(maxIdleConns)
	sqlDB.SetMaxOpenConns(maxOpenConns)

	o.Factory = db.NewDaoFactory(o.DB)
	return nil
}

func (o *Options) registerCicdDriver() error {
	jenkinsOption := o.ComponentConfig.Cicd.Jenkins
	switch o.ComponentConfig.Cicd.Driver {
	case "", types.Jenkins:
		o.CicdDriver = gojenkins.CreateJenkins(nil, jenkinsOption.Host, jenkinsOption.User, jenkinsOption.Password)
		if _, err := o.CicdDriver.Init(context.TODO()); err != nil {
			return err
		}
	}

	return nil
}

// Validate validates all the required options.
// TODO
func (o *Options) Validate() error {
	return nil
}

func (o *Options) Run(stopCh <-chan struct{}) {
	_ = o.GinEngine.Run(fmt.Sprintf(":%d", o.ComponentConfig.Default.Listen))
}
