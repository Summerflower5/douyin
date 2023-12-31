package middleware

import (
	"fmt"
	"time"

	"douyin/database"
	//"douyin/database/models"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/gomodule/redigo/redis"
)

var RedisPool *redis.Pool

func InitRedisPool() {
	// 初始化 Redis 连接池
	RedisPool = &redis.Pool{
		MaxIdle:     10,
		MaxActive:   0,
		IdleTimeout: 240 * time.Second,
		Wait:        true,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", "localhost:6379")
			if err != nil {
				return nil, err
			}
			/*if _, err := c.Do("AUTH", "123456"); err != nil {
				c.Close()
				return nil, err
			}*/
			return c, err
		},
	}
	RedisPool.Get().Do("flushdb")
}

func RedisMiddleware() gin.HandlerFunc {
	InitRedisPool() // 初始化 Redis 连接池
	if RedisPool != nil {
		fmt.Println("Get Redis!")
	}
	LoadMysqlToRedis()
	return func(ctx *gin.Context) {
		ctx.Set("RedisPool", RedisPool) // 将连接池存入上下文
		ctx.Next()
	}
}

func CloseRedis() {
	RedisPool.Close()
}

type UserRedis struct {
	ID             int64
	TotalFavorited int64
	FavoriteCount  int64
}

type VideoRedis struct {
	ID           int64
	AuthorUserID int64
	Likes        int
}

type FavoriteRedis struct {
	UserID  int64
	VideoID int64
}

func LoadMysqlToRedis() {
	//load user
	conn := RedisPool.Get() //重用已有的连接
	defer conn.Close()
	var user []UserRedis
	err := database.DB.Table("user").Select([]string{"id", "total_favorited", "favorite_count"}).Scan(&user).Error
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, u := range user {
		conn.Send("HMSET", "user:"+strconv.FormatInt(u.ID, 10), "total_favorited", u.TotalFavorited, "favorite_count", u.FavoriteCount)
	}
	//load video
	var video []VideoRedis
	err = database.DB.Table("video").Select([]string{"id", "author_user_id", "likes"}).Scan(&video).Error
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, v := range video {
		conn.Send("HMSET", "video:"+strconv.FormatInt(v.ID, 10), "author_user_id", v.AuthorUserID, "likes_count", v.Likes)
	}
	//load favorite
	var favorite []FavoriteRedis
	err = database.DB.Table("favorite").Select([]string{"user_id", "video_id"}).Where("is_deleted=-1").Order("updated_at desc, video_id").Scan(&favorite).Error
	if err != nil {
		fmt.Println(err)
		return
	}
	for _, f := range favorite {
		conn.Send("RPUSH", "user:"+strconv.FormatInt(f.UserID, 10)+":likes", f.VideoID)
	}
	conn.Flush()
	fmt.Println("load cache OK!")
}
