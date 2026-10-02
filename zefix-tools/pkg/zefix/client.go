package zefix

import (
	"context"

	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type Client struct {
	db *gorm.DB
}

var logger = logrus.StandardLogger().WithField("package", "zefix")

func New(dsn string) (*Client, error) {
	return NewWithContext(context.Background(), dsn)
}

func NewWithContext(ctx context.Context, dsn string) (*Client, error) {
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{DisableAutomaticPing: true})
	if err != nil {
		return nil, err
	}

	c := Client{
		db: db.WithContext(ctx),
	}
	if err := c.PingContext(ctx); err != nil {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
		return nil, err
	}
	err = c.initModels()
	if err != nil {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
		return nil, err
	}
	// Initialization deadlines must not remain attached to future queries.
	c.db = db
	return &c, nil
}

func (c *Client) initModels() error {
	err := c.db.AutoMigrate(&Company{})
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) Ping() error {
	return c.PingContext(context.Background())
}

func (c *Client) PingContext(ctx context.Context) error {
	sqlDB, err := c.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.PingContext(ctx)
}

func (c *Client) Close() error {
	sqlDB, err := c.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}
