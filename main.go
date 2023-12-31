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

type Category struct {
	ID   primitive.ObjectID `json:"id" bson:"_id,omitempty"`
	Name string             `json:"name" bson:"name"`
	LowT int                `json:"lowt"`
	OkT  int                `json:"okt"`
}

func DatabaseMiddleware(item_collection *mongo.Collection, cat_collection *mongo.Collection, thresholds map[string]float32) gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Set("collection", item_collection)
		c.Set("thresholds", thresholds)
		c.Set("category-collection", cat_collection)
	}
}

func test_fn(c *gin.Context) {

	c.JSON(http.StatusOK, gin.H{
		"message": "hello, world!",
	})
}

func insert_item(c *gin.Context) {

	capitalizeFirstLetter := func(s string) string {
		if len(s) == 0 {
			return s
		}

		runes := []rune(s)

		if runes[0] >= 65 && runes[0] <= 90 {
			return s
		}

		firstChar := s[0]

		capitalizedFirstChar := 'A' + (firstChar - 'a')
		return string(capitalizedFirstChar) + s[1:]
	}

	var item Item

	if err := c.ShouldBind(&item); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid JSON data",
			"err":   err,
		})
		return
	}
	item.Name = capitalizeFirstLetter(item.Name)
	collection, exists := c.Get("collection")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get database collection",
		})
		return
	}

	thresholdInterface, exists := c.Get("thresholds")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to insert item into database",
		})
		return
	}

	insertedID := result.InsertedID.(primitive.ObjectID)
	c.JSON(http.StatusOK, gin.H{
		"message":    "item inserted successfully",
		"insertedID": insertedID.Hex(),
	})
}

func remove_item(c *gin.Context) {
	itemID := c.Param("id")
	collection, exists := c.Get("collection")
	if !exists {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get database collection",
		})
		return
	}

	objID, err := primitive.ObjectIDFromHex(itemID)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "invalid item ID",
		})
		return
	}

	filter := bson.M{"_id": objID}
	result, err := collection.(*mongo.Collection).DeleteOne(context.Background(), filter)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to delete item from database",
		})
		return
	}

	if result.DeletedCount == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"message": "item not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
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
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": "failed to get database collection",
		})
		return
	}

	findOptions := options.Find()
	findOptions.SetLimit(int64(limit))

	if recentStr == "true" {
		findOptions.SetSort(bson.D{{Key: "time", Value: -1}})
	} else {
		findOptions.SetSort(bson.D{{Key: "time", Value: 1}})
	}

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
		c.JSON(http.StatusOK, gin.H{
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
			c.JSON(http.StatusForbidden, gin.H{
				"message": "there was an error decoding the response",
			})
			return
		}

		items = append(items, item)
	}

	if err := cursor.Err(); err != nil {
		// Handle the error appropriately.
		c.JSON(http.StatusForbidden, gin.H{
			"message": "failed to fetch items",
		})
		return
	}

	c.JSON(http.StatusOK, items)
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

	threeDays := time.Now().UTC().Add(3 * 24 * time.Hour)

	filter := bson.M{
		"expiry": bson.M{
			"$lte": primitive.DateTime(threeDays.UnixNano() / int64(time.Millisecond)),
			"$gt":  primitive.DateTime(0),
		},
	}

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
		c.JSON(http.StatusInternalServerError, gin.H{
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
	if newItem.Quantity != 0 {
		existingItem.Quantity = newItem.Quantity
	}
	if newItem.Location != "" {
		existingItem.Location = newItem.Location
	}

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

func addcategory(c *gin.Context) {
	var newCategory Category
	if err := c.ShouldBind(&newCategory); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid JSON data",
			"err":   err,
		})
		return
	}

}

func getcategories(c *gin.Context) {

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
	items_collection := database.Collection("items-list")
	category_collection := database.Collection("categories")

	r := gin.Default()

	config := cors.DefaultConfig()
	allowed_address := ""
	if mode == "RELEASE" {
		allowed_address = "http://172.16.3.76:600"
	} else {
		allowed_address = "http://localhost:5173"
	}
	config.AllowOrigins = []string{
		allowed_address,
	}

	r.Use(func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", allowed_address)
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS, PATCH")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization")
		c.Writer.Header().Set("Access-Control-Allow-Credentials", "true")

		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	})

	r.Use(cors.New(config))
	r.Use(DatabaseMiddleware(items_collection, category_collection, thresholds))

	r.GET("/hello", test_fn)
	r.POST("/insert", insert_item)
	r.DELETE("/delete/:id", remove_item)
	r.GET("/search", search)
	r.GET("/expiringsoon", expiringsoon)
	r.GET("/lowitems", lowitems)
	r.PATCH("/updateitem/:id", updateitem)
	r.GET("/grocerylist", grocerylist)
	r.GET("/categories", getcategories)
	r.POST("/addcategory", addcategory)

	// Recurring()

	r.Run(":8888")
}
