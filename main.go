package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	// "time"
)

type Item struct {
	ID       primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Name     string             `json:"name"`
	Quantity float32            `json:"quantity"`
	Unit     string             `json:"unit"`
	Location string             `json:"location"`
	Category string             `json:"category"`
	Expiry   string             `json:"expiry"`
	Status   int                `json:"status"`
}

func DatabaseMiddleware(collection *mongo.Collection, thresholds map[string]float32) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("collection", collection)
		c.Set("thresholds", thresholds)
	}
}

func test_fn(c *gin.Context) {
	fmt.Println("test endpoint was hit")

	c.JSON(200, gin.H{
		"message": "hello, world!",
	})
}

func insert_item(c *gin.Context) {
	fmt.Println("hit insert item")

	var item Item

	if err := c.ShouldBind(&item); err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid JSON data",
		})
		return
	}
	item.Status = 0

	collection, exists := c.Get("collection")
	if !exists {
		c.JSON(500, gin.H{
			"error": "failed to get database collection",
		})
		return
	}

	thresholdInterface, exists := c.Get("thresholds")
	if !exists {
		c.JSON(500, gin.H{
			"error": "failed to get thresholds for statuses",
		})
		return
	}
	thresholds := thresholdInterface.(map[string]float32)
	
	if threshold, exists := thresholds[item.Category]; exists {
		if item.Quantity >= threshold*1.3 {
			item.Status = 2
		} else if item.Quantity >= threshold && item.Quantity <= threshold*1.3 {
			item.Status = 1
		} else {
			item.Status = 0
		}
	}
	
	result, err := collection.(*mongo.Collection).InsertOne(context.Background(), item)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "failed to insert item into database",
		})
		return
	}

	insertedID := result.InsertedID.(primitive.ObjectID)
	c.JSON(200, gin.H{
		"message":    "item inserted successfully",
		"insertedID": insertedID.Hex(),
	})
}

func remove_item(c *gin.Context) {
	itemID := c.Param("id")
	collection, exists := c.Get("collection")
	if !exists {
		c.JSON(500, gin.H{
			"error": "failed to get database collection",
		})
		return
	}

	objID, err := primitive.ObjectIDFromHex(itemID)
	if err != nil {
		c.JSON(400, gin.H{
			"error": "invalid item ID",
		})
		return
	}

	filter := bson.M{"_id": objID}
	result, err := collection.(*mongo.Collection).DeleteOne(context.Background(), filter)
	if err != nil {
		c.JSON(500, gin.H{
			"error": "failed to delete item from database",
		})
		return
	}

	if result.DeletedCount == 0 {
		c.JSON(404, gin.H{
			"message": "item not found",
		})
		return
	}

	c.JSON(200, gin.H{
        "message": "Item deleted successfully",
        "itemID":  itemID,
    })
}

func search(c *gin.Context) {

}

func main() {

	err := godotenv.Load()
	if err != nil {
		log.Fatal("error loading .env file:", err)
	}
	conn_string := os.Getenv("CONN_STRING")

	thresholds := map[string]float32{
		"nuts":               0.5, 
		"dals":               1.0, 
		"condiments":         0.2, 
		"spices":             0.1, 
		"fruits":             2.0, 
		"snacks":             0.3, 
		"oils":               0.3, 
		"basic pantry items": 1.0, 
	}

	// Set up the MongoDB client
	clientOptions := options.Client().ApplyURI(conn_string)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	// Check if the connection was successful
	err = client.Ping(context.Background(), nil)
	if err != nil {
		log.Fatal("Error connecting to MongoDB:", err)
	}
	fmt.Println("Connected to MongoDB!")

	database := client.Database("inventory-app")
	collection := database.Collection("items-list")

	r := gin.Default()

	r.Use(DatabaseMiddleware(collection, thresholds))

	r.GET("/hello", test_fn)
	r.POST("/insert", insert_item)
	r.DELETE("/delete/:id", remove_item)
	r.GET("/search", search)

	r.Run(":8080")
}