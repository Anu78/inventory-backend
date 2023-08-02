package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Item struct {
	ID       primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Name     string             `json:"name" bson:"name"`
	Quantity float32            `json:"quantity"`
	Unit     string             `json:"unit"`
	Location string             `json:"location"`
	Category string             `json:"category"`
	Expiry   time.Time          `json:"expiry" bson:"expiry"`
	Status   int                `json:"status"`
	Time     time.Time          `json:"time" bson:"time"`
}

func DatabaseMiddleware(collection *mongo.Collection, thresholds map[string]float32) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("collection", collection)
		c.Set("thresholds", thresholds)
	}
}

func test_fn(c *gin.Context) {

	c.JSON(200, gin.H{
		"message": "hello, world!",
	})
}

func insert_item(c *gin.Context) {

	var item Item

	if err := c.ShouldBind(&item); err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid JSON data",
			"err":   err,
		})
		return
	}

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

	switch item.Category {

	}

	if threshold, exists := thresholds[item.Category]; exists {
		if item.Quantity >= threshold*1.3 {
			item.Status = 2
		} else if item.Quantity >= threshold && item.Quantity <= threshold*1.3 {
			item.Status = 1
		} else {
			item.Status = 0
		}
	}

	item.Time = time.Now()
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
	query := c.Query("query")
	category := c.Query("category")
	location := c.Query("location")
	recentStr := c.Query("recent")

	limit := 20

	collection, exists := c.Get("collection")
	if !exists {
		c.JSON(500, gin.H{
			"error": "failed to get database collection",
		})
		return
	}

	findOptions := options.Find()
	findOptions.SetLimit(int64(limit))

	sortOption := bson.D{}
	if recentStr == "true" {
		sortOption = bson.D{{Key: "time", Value: -1}}
	}

	findOptions.SetSort(sortOption)

	filter := bson.M{}
	if query != "" {
		// Add $regex operator for name field
		filter["name"] = bson.M{"$regex": query}
	}

	if category != "all" {
		// Add $regex operator for category field
		filter["category"] = bson.M{"$regex": category}
	}

	if location != "all" {
		// Add $regex operator for location field
		filter["location"] = bson.M{"$regex": location}
	}

	cursor, err := collection.(*mongo.Collection).Find(context.Background(), filter, findOptions)
	if err != nil {
		c.JSON(200, gin.H{
			"message": "no matching items for that search",
		})
		return
	}
	defer cursor.Close(context.Background())

	var items []Item

	for cursor.Next(context.Background()) {
		var item Item
		if err := cursor.Decode(&item); err != nil {
			// Handle the error appropriately.
			c.JSON(403, gin.H{
				"message": "there was an error decoding the response",
			})
			return
		}

		items = append(items, item)
	}

	if err := cursor.Err(); err != nil {
		// Handle the error appropriately.
		c.JSON(403, gin.H{
			"message": "failed to fetch items",
		})
		return
	}

	c.JSON(200, items)
}

func expiringsoon(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10") // Set a default limit if not provided

	// check if limit is an int
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit > 10 {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Invalid limit argument",
		})
		return
	}

	collection, exists := c.Get("collection")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get database collection",
		})
		return
	}

	// Get objects expiring within three days
	threeDays := time.Now().Add(3 * 24 * time.Hour)
	filter := bson.M{"expiry": bson.M{"$lte": threeDays}}

	cursor, err := collection.(*mongo.Collection).Find(context.Background(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Failed to find items",
			"details": err.Error(),
		})
		return
	}
	defer cursor.Close(context.Background())

	var items []Item
	for cursor.Next(context.Background()) {
		var item Item
		if err := cursor.Decode(&item); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "Failed to decode item",
				"details": err.Error(),
			})
			return
		}
		items = append(items, item)
	}

	if err := cursor.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "Cursor error",
			"details": err.Error(),
		})
		return
	}

	if len(items) == 0 {
		c.JSON(http.StatusOK, []Item{}) // Return an empty array in case of no items
	} else {
		c.JSON(http.StatusOK, items)
	}
}

func lowitems(c *gin.Context) {

}

func updateitem(c *gin.Context) {

	var newItem Item
	if err := c.ShouldBind(&newItem); err != nil {
		c.JSON(400, gin.H{
			"error": "Invalid JSON data",
			"err":   err,
		})
		return
	}

	collection, exists := c.Get("collection")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get database collection",
		})
		return
	}

	objID, err := primitive.ObjectIDFromHex(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "invalid item ID",
			"err":     err,
		})
		return
	}

	filter := bson.M{"_id": objID}

	var existingItem Item
	err = collection.(*mongo.Collection).FindOne(context.Background(), filter).Decode(&existingItem)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "item was not found",
			"err":     err,
		})
		return
	}

	existingItem.Quantity = newItem.Quantity

	_, err = collection.(*mongo.Collection).ReplaceOne(context.Background(), filter, existingItem)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to update item in the database",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "item updated successfully",
		"itemID":  existingItem.ID.Hex(),
	})

}

func grocerylist(c *gin.Context) {

}

func forcelistupdate(c *gin.Context) {
	// UpdateList()
}

func main() {

	logFile, err := os.Create("log.txt")
	if err != nil {
		log.Fatal("failed to create log file", err)
	}

	defer logFile.Close()

	log.SetOutput(logFile)

	logger := logrus.New()
	logger.SetOutput(logFile)

	gin.DefaultWriter = logger.Writer()
	gin.DefaultErrorWriter = logger.Writer()
	err = godotenv.Load()
	if err != nil {
		log.Fatal("error loading .env file:", err)
	}
	conn_string := os.Getenv("CONN_STRING")
	mode := os.Getenv("MODE")

	if mode == "RELEASE" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

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
	serverAPI := options.ServerAPI(options.ServerAPIVersion1)
	clientOptions := options.Client().ApplyURI(conn_string).SetServerAPIOptions(serverAPI)
	client, err := mongo.Connect(context.Background(), clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	// Check if the connection was successful
	err = client.Ping(context.Background(), nil)
	if err != nil {
		log.Fatal("Error connecting to MongoDB:", err)
	}
	log.Println("Connected to MongoDB!")

	database := client.Database("inventory-app")
	collection := database.Collection("items-list")

	r := gin.Default()

	config := cors.DefaultConfig()
	allowed_address := ""
	if mode == "RELEASE" {
		allowed_address = "http://172.16.3.76"
	} else {
		allowed_address = "http://localhost:5173"
	}
	config.AllowOrigins = []string{
		allowed_address,
	}

	r.Use(func(c *gin.Context) {
		// Replace "*" with the specific origin(s) you want to allow
		c.Writer.Header().Set("Access-Control-Allow-Origin", allowed_address)
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204) // No Content
			return
		}

		c.Next()
	})

	r.Use(cors.New(config))
	r.Use(DatabaseMiddleware(collection, thresholds))

	r.GET("/hello", test_fn)
	r.POST("/insert", insert_item)
	r.DELETE("/delete/:id", remove_item)
	r.GET("/search", search)
	r.GET("/expiringsoon", expiringsoon)
	r.GET("/lowitems", lowitems)
	r.PATCH("/updateitem/:id", updateitem)
	r.GET("/grocerylist", grocerylist)
	r.GET("/forcelist", forcelistupdate)

	// Recurring()

	r.Run(":8888")
}
