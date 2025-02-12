package main

import (
	"context"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
const shortURLLength = 6

var collection *mongo.Collection

func initMongoDB() {
	mongoURI := os.Getenv("MONGODB_URI")
	if mongoURI == "" {
		log.Fatal("Error: MONGODB_URI environment variable not set")
	}

	clientOptions := options.Client().ApplyURI(mongoURI)
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		log.Fatal("MongoDB Connection Error: ", err)
	}

	err = client.Ping(context.TODO(), nil)
	if err != nil {
		log.Fatal("MongoDB Ping Failed: ", err)
	}

	log.Println("Connected to MongoDB")
	collection = client.Database("urlshortener").Collection("urls")
}

type URL struct {
	Short  string `bson:"short"`
	Long   string `bson:"long"`
	Clicks int    `bson:"clicks"`
}

func generateShortURL() string {
	short := make([]byte, shortURLLength)
	for i := range short {
		short[i] = charset[rand.Intn(len(charset))]
	}
	return string(short)
}

func shortenURL(c *fiber.Ctx) error {
	reqBody := new(struct {
		Long string `json:"long"`
	})
	if err := c.BodyParser(reqBody); err != nil {
		return c.Status(http.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request"})
	}

	var existing URL
	err := collection.FindOne(context.TODO(), bson.M{"long": reqBody.Long}).Decode(&existing)
	if err == nil {
		return c.JSON(fiber.Map{"short": existing.Short})
	}

	short := generateShortURL()
	doc := URL{Short: short, Long: reqBody.Long, Clicks: 0}
	_, err = collection.InsertOne(context.TODO(), doc)
	if err != nil {
		return c.Status(http.StatusInternalServerError).JSON(fiber.Map{"error": "Database error"})
	}

	return c.JSON(fiber.Map{"short": short})
}

func redirectURL(c *fiber.Ctx) error {
	short := c.Params("short")

	var result URL
	err := collection.FindOneAndUpdate(
		context.TODO(),
		bson.M{"short": short},
		bson.M{"$inc": bson.M{"clicks": 1}},
	).Decode(&result)

	if err != nil {
		return c.Status(http.StatusNotFound).JSON(fiber.Map{"error": "URL not found"})
	}

	return c.Redirect(result.Long, http.StatusFound)
}

func main() {
	rand.Seed(time.Now().UnixNano())
	initMongoDB()

	app := fiber.New()

	app.Post("/shorten", shortenURL)
	app.Get("/:short", redirectURL)

	log.Println("Server running on http://localhost:3000")
	log.Fatal(app.Listen(":3000"))
}
