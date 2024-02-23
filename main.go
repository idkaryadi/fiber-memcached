package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"strconv"
	"sync"

	"github.com/bradfitz/gomemcache/memcache"
	"github.com/gofiber/fiber/v2"
)

type Photo struct {
	AlbumId      int    `json:"albumId"`
	Id           int    `json:"id"`
	ThumbnailUrl string `json:"thumbnailUrl"`
	Title        string `json:"title"`
	Url          string `json:"url"`
}

var mc = memcache.New("0.0.0.0:11211")

func main() {
	app := fiber.New()
	fmt.Println("tambah fmt")
	app.Get("/photo/:id", verifyCache, func(c *fiber.Ctx) error {
		id := c.Params("id")
		res, err := http.Get("https://jsonplaceholder.typicode.com/photos/" + id)
		if err != nil {
			return err
		}

		defer res.Body.Close()
		body, err := io.ReadAll(res.Body)
		if err != nil {
			return err
		}

		// set to cache
		cacheErr := mc.Set(&memcache.Item{Key: id, Value: body, Expiration: 10})
		if cacheErr != nil {
			fmt.Println("err cache::", cacheErr)
		}

		result := toJSON(body)

		return c.JSON(fiber.Map{"data": result})
	})

	// get 10 photos and save it to cache
	app.Get("/photo-list", func(c *fiber.Ctx) error {
		for i := 1; i < 11; i++ {
			id := strconv.Itoa(i)
			res, err := http.Get("https://jsonplaceholder.typicode.com/photos/" + id)
			if err != nil {
				fmt.Println("err hit endpoint::", err)
				// return err
			}

			defer res.Body.Close()
			body, err := io.ReadAll(res.Body)
			if err != nil {
				fmt.Println("err read body::", err)
				// return err
			}

			// set to cache
			cacheErr := mc.Set(&memcache.Item{Key: id, Value: body, Expiration: 10})
			if cacheErr != nil {
				fmt.Println("err cache::", cacheErr)
			}
		}
		return c.JSON(fiber.Map{"message": "success"})
	})

	// get 10 photos and save it to cache use concurrency
	app.Get("/photo-list-concurrency", func(c *fiber.Ctx) error {
		var wg sync.WaitGroup
		for i := 1; i < 11; i++ {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				id := strconv.Itoa(i)
				res, err := http.Get("https://jsonplaceholder.typicode.com/photos/" + id)
				if err != nil {
					fmt.Println("err hit endpoint::", err)
					// return err
				}

				defer res.Body.Close()
				body, err := io.ReadAll(res.Body)
				if err != nil {
					fmt.Println("err read body::", err)
					// return err
				}

				// set to cache
				cacheErr := mc.Set(&memcache.Item{Key: id, Value: body, Expiration: 10})
				if cacheErr != nil {
					fmt.Println("err cache::", cacheErr)
				}
			}(i)
		}

		wg.Wait()
		return c.JSON(fiber.Map{"message": "success"})
	})

	app.Get("/increment", func(c *fiber.Ctx) error {
		cacheErr := mc.Set(&memcache.Item{Key: "counter", Value: []byte(strconv.Itoa(0))})
		if cacheErr != nil {
			fmt.Println("err set cache::", cacheErr)
		}

		runtime.GOMAXPROCS(3)
		var wg sync.WaitGroup
		var mtx sync.Mutex

		// race condition ketika 200 & 3
		// tetep butuh mutex, kalo gak dikasih mutex kayak sering terjadi error
		// error yang muncul: "read tcp 127.0.0.1:57202->127.0.0.1:11211: read: connection reset by peer"
		for i := 0; i < 1000; i++ {
			wg.Add(1)
			go func() {
				for j := 0; j < 100; j++ {
					mtx.Lock()
					_, err := mc.Increment("counter", 1)
					if err != nil {
						fmt.Println("err cache increment::", err)
					}

					mtx.Unlock()
				}
				wg.Done()
			}()
		}

		wg.Wait()
		counterRes, err := mc.Get("counter")
		if err != nil {
			fmt.Println("err get cache::", err)
		}
		fmt.Printf("hahah %d", counterRes.Value)
		fmt.Println("hahah", string(counterRes.Value))
		return c.JSON(fiber.Map{"result": string(counterRes.Value)})
	})
	app.Listen(":3000")
}

func toJSON(body []byte) Photo {
	var result Photo
	err := json.Unmarshal(body, &result)
	if err != nil {
		fmt.Println("err:", err)
	}

	return result
}

// middleware to verify cache
func verifyCache(c *fiber.Ctx) error {
	id := c.Params("id")
	val, err := mc.Get(id)
	if err != nil {
		return c.Next()
	}

	data := toJSON(val.Value)
	return c.JSON(fiber.Map{"cache": data})
}
