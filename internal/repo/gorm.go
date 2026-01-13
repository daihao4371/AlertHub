package repo

import (
	"fmt"
	"reflect"

	"gorm.io/gorm"
)

type GormDBCli struct {
	db *gorm.DB
}

type InterGormDBCli interface {
	Create(table, value interface{}) error
	Update(value Update) error
	Updates(value Updates) error
	Delete(value Delete) error
}

func NewInterGormDBCli(db *gorm.DB) InterGormDBCli {
	return &GormDBCli{
		db: db,
	}
}

// Create 插入数据
// 注意：该方法会自动处理值类型和指针类型，确保传递给 GORM 的是指针
func (g GormDBCli) Create(table, value interface{}) error {
	return g.executeTransaction(func(tx *gorm.DB) error {
		// 检查 value 的类型
		valueType := reflect.TypeOf(value)
		if valueType == nil {
			return fmt.Errorf("插入数据不能为空")
		}

		// 如果 value 是值类型（非指针），需要转换为指针类型
		// 因为 GORM 的 Create 方法需要指针类型才能正确设置默认值
		var createTarget interface{}
		if valueType.Kind() == reflect.Ptr {
			// 已经是指针类型，直接使用
			createTarget = value
		} else {
			// 是值类型，需要创建一个指针并复制数据
			valuePtr := reflect.New(valueType)
			valuePtr.Elem().Set(reflect.ValueOf(value))
			createTarget = valuePtr.Interface()
		}

		return tx.Model(table).Create(createTarget).Error
	}, "数据写入失败")
}

// Update 更新单条数据
func (g GormDBCli) Update(value Update) error {
	return g.executeTransaction(func(tx *gorm.DB) error {
		tx = tx.Model(value.Table)
		for column, val := range value.Where {
			tx = tx.Where(column, val)
		}
		return tx.Update(value.Update[0], value.Update[1:]).Error
	}, "数据更新失败")
}

// Updates 更新多条数据
func (g GormDBCli) Updates(value Updates) error {
	return g.executeTransaction(func(tx *gorm.DB) error {
		tx = tx.Model(value.Table)
		for column, val := range value.Where {
			tx = tx.Where(column, val)
		}
		return tx.Updates(value.Updates).Error
	}, "数据更新失败")
}

// Delete 删除数据
func (g GormDBCli) Delete(value Delete) error {
	return g.executeTransaction(func(tx *gorm.DB) error {
		// 构建查询条件
		tx = tx.Model(value.Table)
		for column, val := range value.Where {
			tx = tx.Where(column, val)
		}

		// GORM Delete 方法需要传入模型实例（指针类型）
		// 如果 value.Table 是值类型，需要转换为指针类型
		var deleteTarget interface{}
		tableType := reflect.TypeOf(value.Table)
		if tableType == nil {
			return fmt.Errorf("删除目标表类型为空")
		}

		// 如果已经是指针类型，直接使用
		if tableType.Kind() == reflect.Ptr {
			deleteTarget = value.Table
		} else {
			// 如果是值类型，创建指针
			tableValue := reflect.New(tableType)
			deleteTarget = tableValue.Interface()
		}

		// 执行删除操作
		result := tx.Delete(deleteTarget)
		if result.Error != nil {
			return result.Error
		}

		return nil
	}, "数据删除失败")
}

// executeTransaction 执行事务并处理错误
func (g GormDBCli) executeTransaction(operation func(tx *gorm.DB) error, errorMessage string) error {
	tx := g.db.Begin()
	if tx.Error != nil {
		return fmt.Errorf("事务启动失败, err: %s", tx.Error)
	}

	if err := operation(tx); err != nil {
		tx.Rollback()
		return fmt.Errorf("%s -> %s", errorMessage, err)
	}

	if err := tx.Commit().Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("事务提交失败, err: %s", err)
	}

	return nil
}

// Update 定义更新单条数据的结构
type Update struct {
	Table  interface{}
	Where  map[string]interface{}
	Update []string
}

// Updates 定义更新多条数据的结构
type Updates struct {
	Table   interface{}
	Where   map[string]interface{}
	Updates interface{}
}

// Delete 定义删除数据的结构
type Delete struct {
	Table interface{}
	Where map[string]interface{}
}
