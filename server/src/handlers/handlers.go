package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	// "io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"strings"

	"github.com/google/generative-ai-go/genai"
	"github.com/google/uuid"
	"github.com/integems/report-agent/config"
	"github.com/integems/report-agent/src/database"
	"github.com/integems/report-agent/src/models"
	"github.com/redis/go-redis/v9"
	"golang.org/x/crypto/bcrypt"

	"github.com/golang-jwt/jwt/v5"
	"google.golang.org/api/option"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var genAIClient *genai.Client

func init() {
	ctx := context.Background()
	var err error
	genAIClient, err = genai.NewClient(ctx, option.WithAPIKey(config.GetEnv("GEMINI_API_KEY", "")))
	if err != nil {
		log.Fatalf("Failed to initialize GenAI client: %v", err)
	}
}

func getGenAIClient() *genai.Client {
	return genAIClient
}

type handler struct {
	mux          *http.ServeMux
	db           *gorm.DB
	rdb          *redis.Client
	redisManager *database.RedisSessionManager
}

// NewHandler initializes a new handler with a mux and database.
func NewHandler(mux *http.ServeMux, db *gorm.DB) *handler {
	redisManager := database.NewRedisSessionManager()
	rdb := database.NewRedisConnection()
	return &handler{mux: mux, db: db, redisManager: redisManager, rdb: rdb}
}

func generateValidFileName(_fileName string) string {

	// Convert UUID to string and make it lowercase
	fileName := strings.ToLower(_fileName)

	// Remove hyphens from the UUID
	fileName = strings.ReplaceAll(fileName, "-", "a")

	// Ensure the filename doesn't start or end with a dash
	if strings.HasPrefix(fileName, "-") || strings.HasSuffix(fileName, "-") {
		fileName = strings.Trim(fileName, "-")
	}
	return fileName
}

// Helper: Hash a password using bcrypt.
func hashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// Generate JWT token
func generateJWT(data map[string]string) (string, error) {

	jwtSecret := config.GetEnv("JWT_SECRET", "integems")
	// Create token claims
	claims := jwt.MapClaims{
		"userId": data["userId"],
		"email":  data["email"],
		"image":  data["image"],
		"name":   data["name"],
		"role":   data["role"],
	}

	// Create token with signing method
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign the token with secret key
	return token.SignedString([]byte(jwtSecret))
}

// Helper: Compare hashed passwords.
func checkPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// Helper function: Get the current working directory
func getWorkingDirectory() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		log.Printf("Failed to get working directory: %v", err)
		return "", err
	}
	return cwd, nil
}

// Delete local file

func deleteLocalFile(filePath string) error {
	err := os.Remove(filePath)
	return err
}

// Helper function: Get or upload the file to GenAI
func (h *handler) getOrUploadFile(ctx context.Context, client *genai.Client, fileName, fileExt string) (*genai.File, error) {
	// Generate a valid file name based on input
	validFileName := generateValidFileName(fileName)
	// Attempt to retrieve the file from the client
	file, err := client.GetFile(ctx, fmt.Sprintf("files/%v", validFileName))
	if err == nil {
		log.Printf("File already exists: %s", file.Name)
		return file, nil
	}

	// Get the working directory
	cwd, err := getWorkingDirectory()
	if err != nil {
		log.Printf("Failed to retrieve working directory: %v", err)
		return nil, err
	}

	// Construct the file path
	filePath := filepath.Join(cwd, "files", fmt.Sprintf("%v%v", validFileName, fileExt))

	opendFile, err := os.Open(filePath)
	if err != nil {
		log.Printf("Failed to open file at path %s: %v", filePath, err)
		return nil, err
	}
	defer opendFile.Close()

	// Upload the file
	log.Printf("Uploading file: %s", validFileName)
	file, err = client.UploadFile(ctx, validFileName, opendFile, nil)
	if err != nil {
		log.Printf("Failed to upload file: %v", err)
		return nil, err
	}

	// Wait for the file to be processed
	for file.State == genai.FileStateProcessing {
		log.Printf("Processing file: %s", file.Name)
		time.Sleep(3 * time.Second) // Polling interval
		file, err = client.GetFile(ctx, file.Name)
		if err != nil {
			log.Printf("Error checking file state: %v", err)
			return nil, err
		}
	}

	// Verify the final state of the file
	if file.State != genai.FileStateActive {
		log.Printf("File state is not active: %s (state: %s)", file.Name, file.State)
		return nil, fmt.Errorf("file not active after processing")
	}
	deleteLocalFile(filePath)
	log.Printf("File successfully processed and ready: %s", file.Name)
	return file, nil
}

func (h *handler) getOrUploadDocFile(ctx context.Context, client *genai.Client, fileName string) (*genai.File, error) {
	// Generate a valid file name based on input
	validFileName := generateValidFileName(fileName)
	// Attempt to retrieve the file from the client
	file, err := client.GetFile(ctx, fmt.Sprintf("files/%v", validFileName))
	if err == nil {
		log.Printf("File already exists: %s", file.Name)
		return file, nil
	}

	var document models.Document
	if err := h.db.Where(&models.Document{DocumentId: fileName}).First(&document).Error; err != nil {
		log.Printf("Failed get file: %v", err)
		return nil, err
	}

	// Get the working directory
	cwd, err := getWorkingDirectory()
	if err != nil {
		log.Printf("Failed to retrieve working directory: %v", err)
		return nil, err
	}

	// Construct the file path
	filePath := filepath.Join(cwd, "documents", fmt.Sprintf("%v.%v", validFileName, document.Type))

	documentFile, err := os.Open(filePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if err := h.db.Where(&models.Document{DocumentId: fileName}).Delete(&document).Error; err != nil {
				log.Printf("Failed to delete file: %v", err)
				return nil, err
			}
		}
		log.Printf("Failed to open file at path %s: %v", filePath, err)
		return nil, err
	}
	defer documentFile.Close()

	// Upload the file
	log.Printf("Uploading file: %s", validFileName)
	file, err = client.UploadFile(ctx, validFileName, documentFile, nil)
	if err != nil {
		log.Printf("Failed to upload file: %v", err)
		return nil, err
	}

	// Wait for the file to be processed
	for file.State == genai.FileStateProcessing {
		log.Printf("Processing file: %s", file.Name)
		time.Sleep(3 * time.Second) // Polling interval
		file, err = client.GetFile(ctx, file.Name)
		if err != nil {
			log.Printf("Error checking file state: %v", err)
			return nil, err
		}
	}

	// Verify the final state of the file
	if file.State != genai.FileStateActive {
		log.Printf("File state is not active: %s (state: %s)", file.Name, file.State)
		return nil, fmt.Errorf("file not active after processing")
	}
	deleteLocalFile(filePath)
	log.Printf("File successfully processed and ready: %s", file.Name)
	return file, nil
}

// Helper function: Get or upload the file to GenAI
func (h *handler) uploadFileToGoogle(ctx context.Context, client *genai.Client, fileName string, docType string) (*genai.File, error) {
	// Generate a valid file name based on input
	validFileName := generateValidFileName(fileName)
	// Get the working directory
	cwd, err := getWorkingDirectory()
	if err != nil {
		log.Printf("Failed to retrieve working directory: %v", err)
		return nil, err
	}

	// Construct the file path
	filePath := filepath.Join(cwd, "documents", fmt.Sprintf("%v.%v", validFileName, docType))

	docFile, err := os.Open(filePath)
	if err != nil {
		log.Printf("Failed to open file at path %s: %v", filePath, err)
		return nil, err
	}
	defer docFile.Close()

	// Upload the file
	log.Printf("Uploading file: %s", validFileName)
	file, err := client.UploadFile(ctx, validFileName, docFile, nil)
	if err != nil {
		log.Printf("Failed to upload file: %v", err)
		return nil, err
	}

	deleteLocalFile(filePath)
	log.Printf("File successfully processed and ready: %s", file.Name)
	return file, nil
}

// Helper function: Respond with an error
func respondWithError(w http.ResponseWriter, message string, code int) {
	errorMessage := struct {
		Message string `json:"message"`
	}{Message: message}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(errorMessage); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
	}
}

// Helper function: Respond with JSON
func respondWithJSON(w http.ResponseWriter, data interface{}, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("Failed to encode JSON response: %v", err)
	}
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////
// Home handler.
func (h *handler) home(w http.ResponseWriter, req *http.Request) {
	responsePayload := map[string]string{"message": "Hello world! Welcome to INTEGEMS documents agents"}
	respondWithJSON(w, responsePayload, http.StatusOK)
}

// Get videos handler.
func (h *handler) getUsers(w http.ResponseWriter, req *http.Request) {
	var users []models.User
	if err := h.db.Find(&users).Error; err != nil {
		http.Error(w, "Failed to fetch users", http.StatusInternalServerError)
		return
	}
	respondWithJSON(w, users, http.StatusOK)
}

// Sign-up handler.
func (h *handler) signUp(w http.ResponseWriter, req *http.Request) {
	var user models.User
	if err := json.NewDecoder(req.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	if user.Password != "" {
		// Hash the password.
		hashedPassword, err := hashPassword(user.Password)
		if err != nil {
			respondWithError(w, "Failed to hash password", http.StatusInternalServerError)
			return
		}
		user.Password = hashedPassword
	}

	user.UserId = uuid.New().String()
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	// Save the user to the database.
	if err := h.db.Clauses(clause.OnConflict{UpdateAll: true}).Create(&user).Error; err != nil {
		respondWithError(w, "Failed to add user. "+err.Error(), http.StatusInternalServerError)
		return
	}

	// responsePayload := map[string]string{"message": "User registered successfully", "userId": user.UserId}
	respondWithJSON(w, user, http.StatusCreated)

}

// Update User handler.
func (h *handler) updateUser(w http.ResponseWriter, req *http.Request) {
	var user models.User
	if err := json.NewDecoder(req.Body).Decode(&user); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Save the user to the database.
	if err := h.db.Where(models.User{UserId: user.UserId}).Updates(&user).Error; err != nil {
		respondWithError(w, "Failed to update user. "+err.Error(), http.StatusInternalServerError)
		return
	}

	responsePayload := map[string]string{"message": "User updated successfully", "userId": user.UserId}
	respondWithJSON(w, responsePayload, http.StatusCreated)

}

// Sign-in handler.
func (h *handler) signIn(w http.ResponseWriter, req *http.Request) {
	var credentials struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	// Decode request body
	if err := json.NewDecoder(req.Body).Decode(&credentials); err != nil {
		respondWithError(w, "Invalid request payload: "+err.Error(), http.StatusBadRequest)
		return
	}

	// Find user by email
	var user models.User
	if err := h.db.Where("email = ?", credentials.Email).First(&user).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			respondWithError(w, "Invalid email or password.", http.StatusUnauthorized)
		} else {
			respondWithError(w, "Database error: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}

	// Verify password
	if !checkPasswordHash(credentials.Password, user.Password) {
		respondWithError(w, "Invalid email or password.", http.StatusUnauthorized)
		return
	}

	// Generate JWT token

	tokenData := map[string]string{"userId": user.UserId, "email": user.Email, "name": user.Name, "image": user.Image, "role": user.Role}
	tokenString, err := generateJWT(tokenData)
	if err != nil {
		respondWithError(w, "Failed to generate token: "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Response with token
	responsePayload := map[string]string{
		"message": "User signed in successfully",
		"token":   tokenString,
	}

	respondWithJSON(w, responsePayload, http.StatusOK)
}

func (h *handler) checkEmail(w http.ResponseWriter, req *http.Request) {
	email := req.PathValue("email")
	if email == "" {
		respondWithError(w, "Email is required", http.StatusBadRequest)
		return
	}
	var user models.User
	if err := h.db.Where("email = ?", email).First(&user).Error; err != nil {
		respondWithJSON(w, "Email does not exist", http.StatusNotFound)
		return
	}
	respondWithJSON(w, map[string]string{"email": email, "userId": user.UserId}, http.StatusOK)
}

func (h *handler) addNewPassword(w http.ResponseWriter, req *http.Request) {
	var request struct {
		UserId      string `json:"userId"`
		NewPassword string `json:"newPassword"`
	}

	// Decode request body
	if err := json.NewDecoder(req.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request payload", http.StatusBadRequest)
		return
	}

	// Validate input
	if request.UserId == "" || request.NewPassword == "" {
		respondWithError(w, "UserId and new password are required", http.StatusBadRequest)
		return
	}

	// Hash the new password
	hashedPassword, err := hashPassword(request.NewPassword)
	if err != nil {
		respondWithError(w, "Failed to hash password", http.StatusInternalServerError)
		return
	}

	// Update the password in the database
	result := h.db.Model(&models.User{}).Where(models.User{UserId: request.UserId}).
		Updates(map[string]interface{}{
			"password": hashedPassword,
		})

	if result.Error != nil {
		respondWithError(w, "Failed to update password. "+result.Error.Error(), http.StatusInternalServerError)
		return
	}

	// Success response
	responsePayload := map[string]string{"message": "Password updated successfully"}
	respondWithJSON(w, responsePayload, http.StatusOK)
}

// Add document handler.
func (h *handler) addDocument(w http.ResponseWriter, req *http.Request) {

	// Limit file size to 1GB
	req.Body = http.MaxBytesReader(w, req.Body, 1*1024*1024*1024)

	if err := req.ParseMultipartForm(1 << 30); err != nil {
		respondWithError(w, "Invalid request format or payload. "+err.Error(), http.StatusBadRequest)
		return
	}

	documentFile, _, err := req.FormFile("file")
	if err != nil {
		respondWithError(w, "File retrival failed. "+err.Error(), http.StatusInternalServerError)
		return
	}
	defer documentFile.Close()

	title := req.FormValue("title")
	userId := req.FormValue("userId")
	fileType := req.FormValue("fileType")

	var document models.Document
	document.DocumentId = uuid.New().String()
	document.CreatedAt = time.Now()
	document.UpdatedAt = time.Now()
	document.UserId = userId
	document.Title = title
	document.Type = fileType

	// Store the pdf file
	cwd, err := getWorkingDirectory()
	if err != nil {
		respondWithError(w, "Couldn't create file. "+err.Error(), http.StatusInternalServerError)
		return
	}

	if err := os.MkdirAll("documents", 0755); err != nil {
		respondWithError(w, "Couldn't create documents directory. "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Create a file to save the video
	filePath := filepath.Join(cwd, "documents", fmt.Sprintf("%v.%v", generateValidFileName(document.DocumentId), document.Type))
	file, err := os.Create(filePath)
	if err != nil {
		respondWithError(w, "Couldn't create file. "+err.Error(), http.StatusInternalServerError)
		return
	}

	defer file.Close()
	// Stream data directly into the file
	if _, err := io.Copy(file, documentFile); err != nil {
		respondWithError(w, "Couldn't save file. "+err.Error(), http.StatusInternalServerError)
		return
	}

	defer os.Remove(filePath)
	// Use the shared GenAI client
	client := getGenAIClient()
	ctx := context.Background()

	uploadedFile, err := h.uploadFileToGoogle(ctx, client, document.DocumentId, document.Type)

	if uploadedFile == nil || err != nil {
		respondWithError(w, "Failed to upload file. "+err.Error(), http.StatusInternalServerError)
		return
	}
	// log.Println("Error", err)
	if err := h.db.Create(&document).Error; err != nil {
		respondWithError(w, "Failed to add file. "+err.Error(), http.StatusInternalServerError)
		return
	}
	respondWithJSON(w, document, http.StatusCreated)
}

// Delete video handler.
func (h *handler) deleteDocument(w http.ResponseWriter, req *http.Request) {

	documentId := req.PathValue("documentId")

	var document models.Document
	if err := h.db.Model(&document).Where(&models.Document{DocumentId: documentId}).Delete(&document).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			respondWithError(w, "File not found.", http.StatusNotFound)
			return
		}
		respondWithError(w, "Failed to delete file. "+err.Error(), http.StatusInternalServerError)
		return
	}
	respondWithJSON(w, map[string]string{"message": "File deleted successfully"}, 203)
}

// Get documents handler.
func (h *handler) getDocuments(w http.ResponseWriter, req *http.Request) {
	var document []models.Document
	if err := h.db.Preload("User").Find(&document).Error; err != nil {
		respondWithError(w, "Failed to fetch document. "+err.Error(), http.StatusInternalServerError)
		return
	}
	respondWithJSON(w, document, http.StatusOK)
}

// Get user document handler.
func (h *handler) getUserDocuments(w http.ResponseWriter, req *http.Request) {
	userId := req.PathValue("userId")
	// Delete inactive document

	var document []models.Document
	if err := h.db.Where(models.Document{UserId: userId}).Find(&document).Error; err != nil {
		respondWithError(w, "Failed to fetch user documents. "+err.Error(), http.StatusInternalServerError)
		return
	}
	respondWithJSON(w, document, http.StatusOK)
}

// Get messages handler.
func (h *handler) getMessages(w http.ResponseWriter, req *http.Request) {
	sessionId := req.PathValue("sessionId")
	var messages []database.Message = []database.Message{}
	messages, err := h.redisManager.GetMessages(sessionId)
	if err != nil {
		respondWithError(w, "Failed to fetch messages. "+err.Error(), http.StatusInternalServerError)
		return
	}
	respondWithJSON(w, messages, http.StatusOK)
}

func (h *handler) chatWithAIDocs(w http.ResponseWriter, req *http.Request) {

	req.Body = http.MaxBytesReader(w, req.Body, 1*1024*1024*1024)

	if err := req.ParseMultipartForm(1 << 30); err != nil {
		respondWithError(w, "Invalid request format or payload. "+err.Error(), http.StatusBadRequest)
		return
	}

	file, fileHeader, err := req.FormFile("file")
	if fileHeader != nil && err != nil {
		respondWithError(w, "File retrieval failed. "+err.Error(), http.StatusInternalServerError)
		return
	}

	text := req.FormValue("text")
	sessionId := req.FormValue("sessionId")
	fileId := uuid.New().String()
	var ext string

	if fileHeader != nil {

		cwd, err := getWorkingDirectory()
		if err != nil {
			respondWithError(w, "Couldn't create file. "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := os.MkdirAll("files", 0755); err != nil {
			respondWithError(w, "Couldn't create files directory. "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Create a file
		ext = filepath.Ext(fileHeader.Filename)
		filePath := filepath.Join(cwd, "files", fmt.Sprintf("%v%v", generateValidFileName(fileId), ext))
		createdFile, err := os.Create(filePath)
		if err != nil {
			respondWithError(w, "Couldn't create file. "+err.Error(), http.StatusInternalServerError)
			return
		}

		defer file.Close()

		// Stream data directly into the file
		if _, err := io.Copy(createdFile, file); err != nil {
			respondWithError(w, "Couldn't save file. "+err.Error(), http.StatusInternalServerError)
			return
		}

		defer os.Remove(filePath)
	}

	// Parse and validate request payload
	// var reqPayload struct {
	// 	Text       string `json:"text"`
	// 	DocumentId string `json:"documentId"`
	// }

	// if err := json.NewDecoder(req.Body).Decode(&reqPayload); err != nil {
	// 	respondWithError(w, "Invalid request payload. "+err.Error(), http.StatusBadRequest)
	// 	return
	// }

	// Initialize Redis session manager
	redisManager := h.redisManager
	ctx := context.Background()

	// Load or initialize chat session
	history, err := redisManager.GetSessionHistory(sessionId)
	if err != nil {
		respondWithError(w, "Internal server error. "+err.Error(), http.StatusInternalServerError)
		return
	}

	// fmt.Println(history)
	// Use the shared GenAI client
	client := getGenAIClient()
	model := client.GenerativeModel("gemini-2.0-flash")
	session := model.StartChat()
	// Get or upload the file
	parentFile, err := h.getOrUploadDocFile(ctx, client, sessionId)
	if err != nil {
		respondWithError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Send message to AI and get response
	history[0] = &genai.Content{Parts: []genai.Part{

		genai.FileData{URI: parentFile.URI},
		genai.Text(`
		Role:
		You are an AI agent specialized in generating professional documents in Microsoft Word-compatible formats (.docx) and PowerPoint presentations (.pptx). Your primary task is to create reports, resumes, or other documents based on user-provided content and a reference document that defines the format. For PowerPoint presentations, you will also generate JSON data compatible with pptxgenjs.
		Instructions:

		1. Format Matching:
		Analyze the structure, style, and formatting of the reference document (e.g., font styles, headings, bullet points, tables, spacing, etc.).
		Ensure the output document strictly follows the same format as the reference document.
		Replicate formatting elements like bullet points, headings, tables, etc., exactly as in the reference.
		Use proper markdown for formating the response

		2. Content Generation:
		Use the data or information provided by the user to generate the document content.
		If the user provides incomplete information, ask clarifying questions to ensure the document meets their needs.
		For resumes, include sections like "Experience," "Education," and "Skills" as per the reference.

		3. Interactivity:
		Be interactive and versatile. Engage with the user to understand their requirements fully.
		Ask questions if instructions are unclear or if additional details are needed (e.g., "What specific data should I include in the report?" or "Should I include a cover letter with the resume?").
		Offer suggestions or improvements to the document if appropriate (e.g., "Would you like me to add a summary section to the report?").

		4. Output:
		Generate the output in a format that can be directly used in Microsoft Word (.docx).
		If the user requests a PowerPoint presentation, provide:
		A text version of the content to show to the user.
		A JSON structure compatible with pptxgenjs for generating the .pptx file.
		Separate the text content and JSON structure with the delimiter &&json.

		5. Versatility:
		Handle a wide range of document types, including:
		Reports (e.g., project reports, financial reports, research papers).
		Resumes and CVs.
		Business proposals.
		Letters and memos.
		PowerPoint presentations.

		Rules:
		Format Priority:
		Always prioritize the format of the reference document provided by the user.
		Remove unnecessary text from the response. For example:

		If the response is:
		AI: 'Here is the report with modification' {Report}
		Remove 'Here is the report with modification' and respond with:
		AI: {Report}

		Presentation JSON:
		For PowerPoint presentations, provide the JSON data at the end of the response, separated by &&json.
		Do not include code or explanations about how to use the JSON with pptxgenjs.
		Ensure the JSON structure is compatible with pptxgenjs and includes all necessary options.
		
		Dynamic Features:
		Use x, y, w, h, and other positioning options to place elements precisely on the slides.
		Include tables, shapes, images, and charts where appropriate.
		Ensure the JSON structure is modern, professional, and visually appealing.
		Use bullet points and other proper formatting for text content.
		Ensure consistency in font styles, colors, and layouts across all slides for a particular presentation and place items (texts,charts,images,shapes) dynamically to ensure properly relative layouts (adjust font-size,relative posive, width,height, and all necessary properties).
		JOSN text design or representation does not have to be same as the text content you user can see,text in the json should be in a format suitable for power point (use pptxgenjs text properties).
		Use default arial font style and make the text font smaller to over going outside the slide. Don't use "-" for  listing and sublisting.

		Measurement Awareness for PowerPoint Slides

		Layout Guidelines for 16:9 Aspect Ratio:

		{
		"slideSize": {
			"width": 10,        // Standard width (inches)
			"height": 5.625     // 16:9 ratio height (inches)
		},
		"safeArea": {
			"x": 0.5,          // Left margin
			"y": 0.5,          // Top margin
			"w": 9,            // Usable width
			"h": 4.625         // Usable height
		}
		}

		Best Practices:
		Keep content within the safe area margins (0.5" from edges)
		Scale text sizes proportionally (32pt for titles, 24pt for subtitles, 18pt for body)
		Position elements using relative coordinates based on 10" × 5.625" dimensions

		Default Slide Dimensions:
		- Widescreen (16:9):** Typically 13.33 inches (width) × 7.5 inches (height)
		- PPTXGenJS Default:** 10 inches (width) × 5.625 inches (height) (16:9 ratio)

		Guidelines for JSON Generation with pptxgenjs:
		1. Coordinate System: 
		Use x, y, w, and h (all in inches) to position and size elements precisely on the slide.

		2. Boundary Checks:
		- Ensure that for each element, x + w does not exceed the slide's width.
		- Ensure that y + h does not exceed the slide's height.
		- If an element would overflow these limits, adjust its size or position accordingly.

		3. Dynamic Scaling and Positioning:
		- Scale elements down or reposition them so that no text, image, shape, table, or chart goes outside the slide boundaries.
		- Consider applying margins or relative positioning adjustments to maintain a professional and cohesive layout.

		4. Consistency:
		- Maintain consistent spacing, font sizes, and alignment across all slides.
		- Use these measurements to ensure that the design is both visually appealing and fully contained within the slide area.

		Theming:

		Match the theme across all slides unless the user specifies otherwise.
		Use consistent fonts, colors, and layouts for a cohesive design.

		RENDERING EQUTION:
		- use latex syntax for equation in text content and don't use latex for pptxgenjs json powerpoint text

		Exmaple for text:
		This is an inline equation: $E = mc^2$.

		Ai: This is a block equation:
			$$
			a^2 + b^2 = c^2
			$$

		Response Format for PowerPoint presentations and/or excel spreadsheet, the response must follow this format:

		{Text Content with no json data}
		&&json
		{JSON Data}

		Example Options for pptxgenjs
		The AI can choose from the following options as an example for each element type but don't forget to modify the properties with suitable custom values for each element to properly represent the text,images,shapes,tables and more relative to the slide window and one another.:

		1. Text:

		{
		"type": "Text",
		"value": "Sample Text",
		"options": {
			"x": 1, // X position (inches)
			"y": 1, // Y position (inches)
			"w": 3, // Width (inches)
			"shape":"ellipse" | "roundRect" | "rect"  | "triangle" | "parallelogram" | "trapezoid" | "diamond"| "pentagon" | "hexagon" | "heptagon" | "octagon" | "decagon" | "dodecagon" | "pie" | "chord" | "teardrop" | "frame" | "halfFrame" | "corner" | "diagStripe" | "plus" | "plaque" | "can" | "cube" | "bevel" | "donut" | "noSmoking" | "blockArc" | "foldedCorner" | "smileyFace" | "heart" | "lightningBolt" | "sun" | "moon" | "cloud" | "arc" | "doubleBracket" | "doubleBrace" | "leftBracket" | "rightBracket"| "leftBrace" | "rightBrace" | "arrow" | "arrowCallout" | "quadArrow" | "leftArrow" | "rightArrow" | "upArrow" | "downArrow" | "leftRightArrow" | "upDownArrow" | "bentArrow" | "uTurnArrow" | "circularArrow" | "leftCircularArrow" | "rightCircularArrow" | "curvedRightArrow"| "curvedLeftArrow" | "curvedUpArrow" | "curvedDownArrow" | "stripedRightArrow" | "notchedRightArrow" | "pentagonArrow" | "chevron" | "leftRightChevron" | "star4" | "star5" | "star6" | "star7" | "star8" | "star10" | "star12" | "star16" | "star24" | "star32" | "ribbon" | "ribbon2" | "banner" | "wavyBanner" | "callout" | "rectCallout" | "roundRectCallout" | "ellipseCallout" | "cloudCallout" | "lineCallout" | "quadArrowCallout" | "leftArrowCallout" | "rightArrowCallout" | "upArrowCallout" | "downArrowCallout" | "leftRightArrowCallout" | "upDownArrowCallout" | "bentArrowCallout" | "uTurnArrowCallout" | "circularArrowCallout" | "leftCircularArrowCallout" | "rightCircularArrowCallout" | "curvedRightArrowCallout" | "curvedLeftArrowCallout" | "curvedUpArrowCallout" | "curvedDownArrowCallout" // If a shape with a text inside the shape is required
			"h": 1, // Height (inches)
			"fontSize": 24, // Font size (points)
			"fontFace": "Arial", // Font family
			"bold": true, // Bold text
			"italic": false, // Italic text
			"underline": false, // Underline text
			"color": "FFFFFF", // Text color (hex)
			"align": "center", // Text alignment (left, center, right)
			"valign": "middle", // Vertical alignment (top, middle, bottom)
			"fill": { "color": "000000" }, // Background color (hex)
			"margin": 0.1, // Margin (inches)
			"lineSpacing": 1.5, // Line spacing
			"charSpacing": 0, // Character spacing
			"bullet": false, // Enable bullet points
			"paraSpaceAfter": 0, // Space after paragraph (inches)
			"paraSpaceBefore": 0 // Space before paragraph (inches)
			}
		}

		2. Chart:

		for chart value only choose from the options = "line" | "pie" | "area" | "bar" | "bar3D" | "bubble" | "doughnut" | "radar" | "scatter"

		{
		"type": "Chart",
		"value":"line" | "pie" | "area" | "bar" | "bar3D" | "bubble" | "doughnut" | "radar" | "scatter",
		"options": {
			"x": 1, // X position (inches)
			"y": 2, // Y position (inches)
			"w": 6, // Width (inches)
			"h": 4, // Height (inches)
			"chartColors": ["FF0000", "00FF00", "0000FF"], // Chart colors (hex)
			"chartColorsOpacity": 50, // Chart colors opacity (0-100)
			"title": "Sales Report", // Chart title
			"showLegend": true, // Show legend
			"legendPos": "r", // Legend position (b, t, l, r)
			"showTitle": true, // Show chart title
			"showValue": true, // Show data values
			"dataLabelFormatCode": "#,##0", // Data label format
			"catAxisLabelColor": "000000", // Category axis label color (hex)
			"valAxisLabelColor": "000000", // Value axis label color (hex)
			"catGridLine": { "color": "CCCCCC", "width": 1 }, // Category grid line
			"valGridLine": { "color": "CCCCCC", "width": 1 } // Value grid line
		}
	}


		3. Image:
	
		{
		"type": "Image",
		"value": "image.png", // Image path or URL
		"options": {
			"x": 1, // X position (inches)
			"y": 6, // Y position (inches)
			"w": 4, // Width (inches)
			"h": 3, // Height (inches)
			"hyperlink": { "url": "https://example.com" }, // Hyperlink
			"rounding": true, // Round corners
			"sizing": { "type": "cover", "w": 4, "h": 3 }, // Image sizing
			"placeholder": "Click to add image" // Placeholder text
		}
		}

		4. Table:

		{
		"type": "Table",
		"value": [ // Table data (2D array)
			["Name", "Age", "City"],
			["John", "30", "New York"],
			["Jane", "25", "Los Angeles"]
		],
		"options": {
			"x": 1, // X position (inches)
			"y": 9, // Y position (inches)
			"w": 6, // Width (inches)
			"h": 2, // Height (inches)
			"colW": [2, 2, 2], // Column widths (inches)
			"rowH": [0.5, 0.5, 0.5], // Row heights (inches)
			"border": { "pt": 1, "color": "000000" }, // Border properties
			"fill": { "color": "F0F0F0" }, // Background color (hex)
			"fontSize": 12, // Font size (points)
			"fontFace": "Arial", // Font family
			"color": "000000", // Text color (hex)
			"align": "center", // Text alignment (left, center, right)
			"valign": "middle", // Vertical alignment (top, middle, bottom)
			"margin": 0.1, // Margin (inches)
			"autoPage": true // Enable auto-pagination
		}
		}

		5. Shape:
				For shape value choose only from the options = "ellipse" | "roundRect" | "rect"  | "triangle" | "parallelogram" | "trapezoid" | "diamond"| "pentagon" | "hexagon" | "heptagon" | "octagon" | "decagon" | "dodecagon" | "pie" | "chord" | "teardrop" | "frame" | "halfFrame" | "corner" | "diagStripe" | "plus" | "plaque" | "can" | "cube" | "bevel" | "donut" | "noSmoking" | "blockArc" | "foldedCorner" | "smileyFace" | "heart" | "lightningBolt" | "sun" | "moon" | "cloud" | "arc" | "doubleBracket" | "doubleBrace" | "leftBracket" | "rightBracket"| "leftBrace" | "rightBrace" | "arrow" | "arrowCallout" | "quadArrow" | "leftArrow" | "rightArrow" | "upArrow" | "downArrow" | "leftRightArrow" | "upDownArrow" | "bentArrow" | "uTurnArrow" | "circularArrow" | "leftCircularArrow" | "rightCircularArrow" | "curvedRightArrow"| "curvedLeftArrow" | "curvedUpArrow" | "curvedDownArrow" | "stripedRightArrow" | "notchedRightArrow" | "pentagonArrow" | "chevron" | "leftRightChevron" | "star4" | "star5" | "star6" | "star7" | "star8" | "star10" | "star12" | "star16" | "star24" | "star32" | "ribbon" | "ribbon2" | "banner" | "wavyBanner" | "callout" | "rectCallout" | "roundRectCallout" | "ellipseCallout" | "cloudCallout" | "lineCallout" | "quadArrowCallout" | "leftArrowCallout" | "rightArrowCallout" | "upArrowCallout" | "downArrowCallout" | "leftRightArrowCallout" | "upDownArrowCallout" | "bentArrowCallout" | "uTurnArrowCallout" | "circularArrowCallout" | "leftCircularArrowCallout" | "rightCircularArrowCallout" | "curvedRightArrowCallout" | "curvedLeftArrowCallout" | "curvedUpArrowCallout" | "curvedDownArrowCallout"

		{
		"type": "Shape",
		"value": "lineCallout",
		"options": {
			"x": 1, // X position (inches)
			"y": 11, // Y position (inches)
			"w": 4, // Width (inches)
			"h": 2, // Height (inches)
			"fill": { "color": "FF0000" }, // Fill color (hex)
			"line": { "color": "000000", "width": 1 }, // Line color and width
			"shadow": { "type": "outer", "color": "000000", "blur": 3 }, // Shadow
			"fontSize": 14, // Font size (points)
			"fontFace": "Arial", // Font family
			"color": "FFFFFF", // Text color (hex)
			"align": "center", // Text alignment (left, center, right)
			"valign": "middle", // Vertical alignment (top, middle, bottom)
			"rotate": 0 // Rotation angle (degrees)
		}
		}
		}

		TYPES DEFINITION FOR PPPTXGENJS:

		type ContentType = "Image" | "Shape" | "Table" | "Text" | "Chart";
		type TableRows = PptxGenJS.TableRow[];
		type DataValue =
		| string
		| TableRows
		| PptxGenJS.SHAPE_NAME
		| PptxGenJS.CHART_NAME;

		type DataOption =
		| PptxGenJS.TextPropsOptions
		| PptxGenJS.ImageProps
		| PptxGenJS.TableProps
		| PptxGenJS.ShapeProps
		| PptxGenJS.IChartOpts;


		type ChatData = {
					name:string,
					labels:string[],
					values:string|number[] 
				};

		type SlideData = {
			type: ContentType;
			value: DataValue;
			options?: DataOption;
			chatData?: ChatData[]; // for chart
		};

		type SlideContent = {
			data: SlideData[];
			};

			
		EXECEL:

		If you are required or ask for excel. You can provide the data in the format in the example below:
		Types format:

		type ExcelData = {
			columnLabels: string[];
			rowLabels: string[];
			data: any[];
		} (use this format for single excel and don't use the below format)

		DO NOT USE:
		type ExcelData = { someName: {
			columnLabels: string[];
			rowLabels: string[];
			data: any[];
		}} (Don't use this format)
			
		Example Interaction
		User Request:
		"Create a PowerPoint presentation about climate change. Use the format of the uploaded document and include the following data: [data provided] and provide an excel data"

		AI Response (Strictly markdown format and space sections or contents properly,make it nicer according to how proper it will fit the power points):

		### Table: Excel Table
		| Order ID | Customer | Employee | Ship Name | City | Address |
		|----------|----------|----------|-----------|------|---------|
		| 10248    | VINET    | 5        | Vins et alcools Chevalier | Reims | 59 rue de lAbbaye |
		| 10249    | TOMSP    | 6        | Toms Spezialitäten | Münster | Luisenstr. 48 |
		| 10250    | HANAR    | 4        | Hanari Carnes | Rio de Janeiro | Rua do Paço, 67 |
		| 10251    | VICTE    | 3        | Victuailles en stock | Lyon | 2, rue du Commerce |
		
		## Slide 1: Title Slide
		### Climate Change: Causes and Effects  
		### A Comprehensive Overview  
		**Author:** John Doe

		## Slide 2: Introduction
		### What is Climate Change?  
		Climate change refers to long-term shifts in temperatures and weather patterns...

		![Climate Change](/powerpoint.jpg)  
		*Positioned at (x:1, y:2.8, w:4, h:2.5)*


		## Slide 3: Causes of Climate Change
		### Key Causes  
		- Greenhouse gas emissions  
		- Deforestation  
		- Industrial activities  

		### Table: Cause vs. Impact Level

		| Cause                 | Impact Level |
		|-----------------------|--------------|
		| Greenhouse gases      | High         |
		| Deforestation         | Medium       |
		| Industrial activities | High         |


		## Slide 4: Effects of Climate Change
		### Key Effects  
		- Rising global temperatures  
		- Melting ice caps  
		- Increased frequency of extreme weather events  

		**Shape:** Rectangle with text *"Urgent Action Needed"*  
		*Positioned at (x:2, y:4, w:6, h:1)*
	
		## Slide 5: Conclusion
		### What Can We Do?  
		Addressing climate change requires global cooperation...

		**Chart:** Bar chart showing CO₂ emissions by country  
		*Positioned at (x:1, y:2.8, w:8, h:2.5)*

		&&json

		'''json
		{
		"excel": {
			"columnLabels": [
			"Order ID",
			"Customer",
			"Employee",
			"Ship Name",
			"City",
			"Address"
			],
			"rowLabels": [
			"Row 1",
			"Row 2",
			"Row 3",
			"Row 4",
			"Row 5",
			"Row 6",
			"Row 7",
			"Row 8",
			"Row 9",
			"Row 10"
			],
			"data": [
			[
				{ "value": 10248 },
				{ "value": "VINET" },
				{ "value": 5 },
				{ "value": "Vins et alcools Chevalier" },
				{ "value": "Reims" },
				{ "value": "59 rue de lAbbaye" }
			],
			[
				{ "value": 10249 },
				{ "value": "TOMSP" },
				{ "value": 6 },
				{ "value": "Toms Spezialitäten" },
				{ "value": "Münster" },
				{ "value": "Luisenstr. 48" }
			],
			[
				{ "value": 10250 },
				{ "value": "HANAR" },
				{ "value": 4 },
				{ "value": "Hanari Carnes" },
				{ "value": "Rio de Janeiro" },
				{ "value": "Rua do Paço, 67" }
			],
			[
				{ "value": 10251 },
				{ "value": "VICTE" },
				{ "value": 3 },
				{ "value": "Victuailles en stock" },
				{ "value": "Lyon" },
				{ "value": "2, rue du Commerce" }
			],
			[
				{ "value": 10252 },
				{ "value": "SUPRD" },
				{ "value": 4 },
				{ "value": "Suprêmes délices" },
				{ "value": "Charleroi" },
				{ "value": "Boulevard Tirou, 255" }
			],
			[
				{ "value": 10253 },
				{ "value": "ALFKI" },
				{ "value": 7 },
				{ "value": "Alfreds Futterkiste" },
				{ "value": "Berlin" },
				{ "value": "Obere Str. 57" }
			],
			[
				{ "value": 10254 },
				{ "value": "FRANK" },
				{ "value": 1 },
				{ "value": "Frankenversand" },
				{ "value": "Mannheim" },
				{ "value": "Berliner Platz 43" }
			],
			[
				{ "value": 10255 },
				{ "value": "BLONP" },
				{ "value": 2 },
				{ "value": "Blondel père et fils" },
				{ "value": "Strasbourg" },
				{ "value": "24, place Kléber" }
			],
			[
				{ "value": 10256 },
				{ "value": "FOLKO" },
				{ "value": 8 },
				{ "value": "Folk och fä HB" },
				{ "value": "Bräcke" },
				{ "value": "Åkergatan 24" }
			],
			[
				{ "value": 10257 },
				{ "value": "MEREP" },
				{ "value": 9 },
				{ "value": "Mère Paillarde" },
				{ "value": "Montréal" },
				{ "value": "43 rue St. Laurent" }
			]
			]
		},
		"slides": [
			{
			"data": [
				{
				"type": "Text",
				"value": "Climate Change: Causes and Effects",
				"options": {
					"fontSize": 32,
					"bold": true,
					"align": "center",
					"x": 0.5,
					"y": 1.5,
					"w": 9,
					"h": 0.8,
					"color": "363636"
				}
				},
				{
				"type": "Text",
				"value": "A Comprehensive Overview",
				"options": {
					"fontSize": 18,
					"align": "center",
					"x": 0.5,
					"y": 2.5,
					"w": 9,
					"h": 0.5,
					"color": "666666"
				}
				},
				{
				"type": "Text",
				"value": "John Doe",
				"options": {
					"fontSize": 14,
					"align": "center",
					"x": 0.5,
					"y": 3.2,
					"w": 9,
					"h": 0.4,
					"color": "666666"
				}
				}
			]
			},
			{
			"data": [
				{
				"type": "Text",
				"value": "What is Climate Change?",
				"options": {
					"fontSize": 28,
					"bold": true,
					"x": 0.5,
					"y": 0.4,
					"w": 9,
					"h": 0.6,
					"color": "363636"
				}
				},
				{
				"type": "Text",
				"value": "Climate change refers to long-term shifts in temperatures and weather patterns...",
				"options": {
					"fontSize": 16,
					"x": 0.5,
					"y": 1.2,
					"w": 9,
					"h": 0.8,
					"color": "666666"
				}
				},
				{
				"type": "Image",
				"value": "/powerpoint.jpg",
				"options": {
					"x": 2,
					"y": 2.2,
					"w": 6,
					"h": 3,
					"sizing": { "type": "contain" }
				}
				}
			]
			},
			{
			"data": [
				{
				"type": "Text",
				"value": "Key Causes",
				"options": {
					"fontSize": 28,
					"bold": true,
					"x": 0.5,
					"y": 0.4,
					"w": 9,
					"h": 0.6,
					"color": "363636"
				}
				},
				{
				"type": "Table",
				"value": [
					[
					{
						"text": "Cause",
						"options": {
						"color": "FFFFFF",
						"fill": { "color": "C00000" }, // Changed to red
						"bold": true,
						"fontSize": 14
						}
					}
					],
					[
					{ "text": "Greenhouse gases", "options": { "fontSize": 14 } },
					{ "text": "High", "options": { "fontSize": 14 } }
					],
					[
					{ "text": "Deforestation", "options": { "fontSize": 14 } },
					{ "text": "Medium", "options": { "fontSize": 14 } }
					],
					[
					{ "text": "Industrial activities", "options": { "fontSize": 14 } },
					{ "text": "High", "options": { "fontSize": 14 } }
					]
				],
				"options": {
					"x": 2,
					"y": 1.4,
					"w": 6,
					"h": 2.5,
					"colW": [3, 3],
					"border": { "pt": 0.5, "color": "C00000" }, // Changed to red
					"align": "center",
					"valign": "middle"
				}
				}
			]
			},
			{
			"data": [
				{
				"type": "Text",
				"value": "Key Effects",
				"options": {
					"fontSize": 28,
					"bold": true,
					"x": 0.5,
					"y": 0.4,
					"w": 9,
					"h": 0.6,
					"color": "363636"
				}
				},
				{
				"type": "Text",
				"value": "Rising global temperatures",
				"options": {
					"fontSize": 16,
					"x": 1,
					"y": 1.2,
					"w": 8,
					"h": 0.4,
					"bullet": true,
					"color": "666666",
					"align": "left",
					"valign": "middle"
				}
				},
				{
				"type": "Text",
				"value": "Melting ice caps",
				"options": {
					"fontSize": 16,
					"x": 1,
					"y": 1.8,
					"w": 8,
					"h": 0.4,
					"bullet": true,
					"color": "666666",
					"align": "left",
					"valign": "middle"
				}
				},
				{
				"type": "Text",
				"value": "Increased frequency of extreme weather events",
				"options": {
					"fontSize": 16,
					"x": 1,
					"y": 2.4,
					"w": 8,
					"h": 0.4,
					"bullet": true,
					"color": "666666",
					"align": "left",
					"valign": "middle"
				}
				},
				{
				"type": "Text",
				"value": "Urgent Action Needed",
				"options": {
					"shape": "roundRect",
					"x": 2.5,
					"y": 3.5,
					"w": 5,
					"h": 0.8,
					"fill": { "color": "C00000" }, // Using red
					"line": { "color": "FFFFFF", "width": 1 },
					"fontSize": 16,
					"color": "FFFFFF",
					"bold": true,
					"align": "center",
					"valign": "middle"
				}
				}
			]
			},
			{
			"data": [
				{
				"type": "Text",
				"value": "What Can We Do?",
				"options": {
					"fontSize": 28,
					"bold": true,
					"x": 0.5,
					"y": 0.4,
					"w": 9,
					"h": 0.6,
					"color": "363636"
				}
				},
				{
				"type": "Text",
				"value": "Addressing climate change requires global cooperation...",
				"options": {
					"fontSize": 16,
					"x": 0.5,
					"y": 1.2,
					"w": 9,
					"h": 0.6,
					"color": "666666"
				}
				},
				{
				"type": "Chart",
				"value": "bar",
				"chatData": [
					{
					"name": "CO₂ Emissions",
					"labels": ["USA", "China", "India"],
					"values": [15, 28, 7]
					}
				],
				"options": {
					"x": 1,
					"y": 2,
					"w": 8,
					"h": 3,
					"showTitle": true,
					"showValue": true,
					"chartColors": ["C00000"], // Changed to red
					"fontSize": 14,
					"legendPos": "b",
					"catGridLine": { "color": "C00000", "width": 1 }, // Changed to red
					"valGridLine": { "color": "C00000", "width": 1 } // Changed to red
				}
				}
			]
			},
			{
			"data": [
				{
				"type": "Text",
				"value": "Climate Change Tree Diagram",
				"options": {
					"fontSize": 28,
					"bold": true,
					"align": "center",
					"x": 0.5,
					"y": 0.2,
					"w": 9,
					"h": 0.6,
					"color": "363636"
				}
				},
				{
				"type": "Text",
				"value": "Climate Change",
				"options": {
					"shape": "ellipse",
					"x": 4,
					"y": 1,
					"w": 2,
					"h": 0.8,
					"fill": { "color": "EFEFEF" },
					"line": { "color": "000000", "width": 1 },
					"fontSize": 12,
					"color": "000000",
					"align": "center",
					"valign": "middle"
				}
				},
				{
				"type": "Text",
				"value": "Causes",
				"options": {
					"shape": "roundRect",
					"x": 1,
					"y": 2,
					"w": 2,
					"h": 0.8,
					"fill": { "color": "EFEFEF" },
					"line": { "color": "000000", "width": 1 },
					"fontSize": 12,
					"color": "000000",
					"align": "center",
					"valign": "middle"
				}
				},
				{
				"type": "Text",
				"value": "Effects",
				"options": {
					"shape": "roundRect",
					"x": 7,
					"y": 2,
					"w": 2,
					"h": 0.8,
					"fill": { "color": "EFEFEF" },
					"line": { "color": "000000", "width": 1 },
					"fontSize": 12,
					"color": "000000",
					"align": "center",
					"valign": "middle"
				}
				},
				{
				"type": "Text",
				"value": "Greenhouse Gases",
				"options": {
					"shape": "roundRect",
					"x": 0.7,
					"y": 3,
					"w": 1.2,
					"h": 0.8,
					"fill": { "color": "FFFFFF" },
					"line": { "color": "000000", "width": 1 },
					"fontSize": 10,
					"color": "000000",
					"align": "center",
					"valign": "middle"
				}
				},
				{
				"type": "Text",
				"value": "Deforestation",
				"options": {
					"shape": "roundRect",
					"x": 1.9,
					"y": 3,
					"w": 1.2,
					"h": 0.8,
					"fill": { "color": "FFFFFF" },
					"line": { "color": "000000", "width": 1 },
					"fontSize": 10,
					"color": "000000",
					"align": "center",
					"valign": "middle"
				}
				},
				{
				"type": "Text",
				"value": "Industrial Activities",
				"options": {
					"shape": "roundRect",
					"x": 3.1,
					"y": 3,
					"w": 1.8,
					"h": 0.8,
					"fill": { "color": "FFFFFF" },
					"line": { "color": "000000", "width": 1 },
					"fontSize": 10,
					"color": "000000",
					"align": "center",
					"valign": "middle"
				}
				},
				{
				"type": "Text",
				"value": "Global Warming",
				"options": {
					"shape": "roundRect",
					"x": 5.5,
					"y": 3,
					"w": 1.5,
					"h": 0.8,
					"fill": { "color": "FFFFFF" },
					"line": { "color": "000000", "width": 1 },
					"fontSize": 10,
					"color": "000000",
					"align": "center",
					"valign": "middle"
				}
				},
				{
				"type": "Text",
				"value": "Extreme Weather",
				"options": {
					"shape": "roundRect",
					"x": 7.1,
					"y": 3,
					"w": 1.5,
					"h": 0.8,
					"fill": { "color": "FFFFFF" },
					"line": { "color": "000000", "width": 1 },
					"fontSize": 10,
					"color": "000000",
					"align": "center",
					"valign": "middle"
				}
				},
				{
				"type": "Text",
				"value": "Rising Sea Levels",
				"options": {
					"shape": "roundRect",
					"x": 8.7,
					"y": 3,
					"w": 1.5,
					"h": 0.8,
					"fill": { "color": "FFFFFF" },
					"line": { "color": "000000", "width": 1 },
					"fontSize": 10,
					"color": "000000",
					"align": "center",
					"valign": "middle"
				}
				},
				{
				"type": "Shape",
				"value": "lineCallout",
				"options": {
					"x": 3.5,
					"y": 1.9,
					"w": 3.16,
					"h": 0.1,
					"rotate": -18.4,
					"line": { "color": "000000", "width": 1 }
				}
				},
				{
				"type": "Shape",
				"value": "lineCallout",
				"options": {
					"x": 6.2,
					"y": 1.9,
					"w": 3.16,
					"h": 0.1,
					"rotate": 18.4,
					"line": { "color": "000000", "width": 1 }
				}
				},
				{
				  "type": "Shape",
				  "value": "lineCallout",
				  "options": {
					"x": 2.75,
					"y": 2.4,
					"w": 0.25,
					"h": 0.1,
					"rotate": 0,
					"line": {
					  "color": "000000",
					  "width": 1,
					  "beginArrowType": "none",
					  "endArrowType": "arrow" 
					},
					"shapeName": "flowchart-arrow-1",
					"flipH": false
				  }
				},
				{
				  "type": "Shape", 
				  "value": "lineCallout",
				  "options": {
					"x": 5,
					"y": 2.4,
					"w": 0.25,
					"h": 0.1,
					"rotate": 0,
					"line": {
					  "color": "000000",
					  "width": 1,
					  "beginArrowType": "none",
					  "endArrowType": "arrow"
					},
					"shapeName": "flowchart-arrow-2",
					"flipH": false
				  }
				},
				{
				  "type": "Shape",
				  "value": "lineCallout",
				  "options": {
					"x": 7.25,
					"y": 2.4,
					"w": 0.25,
					"h": 0.1,
					"rotate": 0,
					"line": {
					  "color": "000000",
					  "width": 1,
					  "beginArrowType": "none",
					  "endArrowType": "arrow"
					},
					"shapeName": "flowchart-arrow-3",
					"flipH": false
				  }
				},
				{
				  "type": "Shape",
				  "value": "rightArrow",
				  "options": {
					"x": 2.75,
					"y": 2.2,
					"w": 0.75,
					"h": 0.4,
					"fill": { "color": "C00000" }, // Using red
					"line": { "color": "FFFFFF", "width": 1 },
					"flipH": false,
					"rotate": 0
				  }
				},
				{
				  "type": "Shape", 
				  "value": "rightArrow",
				  "options": {
					"x": 5,
					"y": 2.2,
					"w": 0.75,
					"h": 0.4,
					"fill": { "color": "C00000" }, // Using red
					"line": { "color": "FFFFFF", "width": 1 },
					"flipH": false,
					"rotate": 0
				  }
				},
				{
				  "type": "Shape",
				  "value": "rightArrow",
				  "options": {
					"x": 7.25,
					"y": 2.2,
					"w": 0.75,
					"h": 0.4,
					"fill": { "color": "C00000" }, // Using red
					"line": { "color": "FFFFFF", "width": 1 },
					"flipH": false,
					"rotate": 0
				  }
				},
				{
				  "type": "Shape",
				  "value": "lineCallout",
				  "options": {
					"x": 2,
					"y": 1.4,
					"w": 2,
					"h": 0.6,
					"rotate": -45,
					"line": {
					  "color": "000000",
					  "width": 1,
					  "beginArrowType": "none",
					  "endArrowType": "arrow"
					}
				  }
				},
				{
				  "type": "Shape",
				  "value": "lineCallout",
				  "options": {
					"x": 6,
					"y": 1.4,
					"w": 2,
					"h": 0.6,
					"rotate": 45,
					"line": {
					  "color": "000000", 
					  "width": 1,
					  "beginArrowType": "none",
					  "endArrowType": "arrow"
					}
				  }
				},
				{
				  "type": "Shape",
				  "value": "rightArrow",
				  "options": {
					"x": 0.7,
					"y": 2.8,
					"w": 0.5,
					"h": 0.2,
					"fill": { "color": "C00000" },
					"line": { "color": "FFFFFF", "width": 1 },
					"rotate": 90
				  }
				},
				{
				  "type": "Shape", 
				  "value": "rightArrow",
				  "options": {
					"x": 1.9,
					"y": 2.8,
					"w": 0.5,
					"h": 0.2,
					"fill": { "color": "C00000" },
					"line": { "color": "FFFFFF", "width": 1 },
					"rotate": 90
				  }
				},
				{
				  "type": "Shape",
				  "value": "rightArrow", 
				  "options": {
					"x": 3.1,
					"y": 2.8,
					"w": 0.5,
					"h": 0.2,
					"fill": { "color": "C00000" },
					"line": { "color": "FFFFFF", "width": 1 },
					"rotate": 90
				  }
				},
				// For the Tree Diagram:
				// Adjust connector arrows and line positions
				{
				  "type": "Shape",
				  "value": "lineCallout",
				  "options": {
					"x": 3.0,
					"y": 1.4,
					"w": 2.0,
					"h": 0.8,
					"rotate": -25,
					"line": {
					  "color": "000000",
					  "width": 1,
					  "beginArrowType": "none", 
					  "endArrowType": "arrow"
					}
				  }
				},
				{
				  "type": "Shape", 
				  "value": "lineCallout",
				  "options": {
					"x": 5.0,
					"y": 1.4,
					"w": 2.0,
					"h": 0.8,
					"rotate": 25,
					"line": {
					  "color": "000000",
					  "width": 1,
					  "beginArrowType": "none",
					  "endArrowType": "arrow"  
					}
				  }
				},
				// Vertical connectors for sub-nodes
				{
				  "type": "Shape",
				  "value": "lineCallout",
				  "options": {
					"x": 0.85,
					"y": 2.8,
					"w": 0.1,
					"h": 0.2,
					"rotate": 90,
					"line": {
					  "color": "000000",
					  "width": 1,
					  "beginArrowType": "none",
					  "endArrowType": "arrow"
					}
				  }
				},
				{
				  "type": "Shape",
				  "value": "lineCallout", 
				  "options": {
					"x": 2.0,
					"y": 2.8,
					"w": 0.1,
					"h": 0.2,
					"rotate": 90,
					"line": {
					  "color": "000000",
					  "width": 1,
					  "beginArrowType": "none",
					  "endArrowType": "arrow"
					}
				  }
				},
				{
				  "type": "Shape",
				  "value": "lineCallout",
				  "options": {
					"x": 3.2,
					"y": 2.8,
					"w": 0.1,
					"h": 0.2, 
					"rotate": 90,
					"line": {
					  "color": "000000",
					  "width": 1,
					  "beginArrowType": "none",
					  "endArrowType": "arrow"
					}
				  }
				}
			]
			},
			{
			"data": [
				{
				"type": "Text",
				"value": "Climate Change Flowchart",
				"options": {
					"fontSize": 28,
					"bold": true,
					"align": "center",
					"x": 0.5,
					"y": 0.2,
					"w": 9,
					"h": 0.6,
					"color": "363636"
				}
				},
				{
				"type": "Text",
				"value": "Start",
				"options": {
					"shape": "roundRect",
					"x": 0.75,
					"y": 2.0,
					"w": 1.5,
					"h": 0.7,
					"fill": {"color": "EFEFEF"},
					"line": {"color": "C00000", "width": 1},
					"fontSize": 12
				}
				},
				{
				"type": "Text",
				"value": "Analyze Data",
				"options": {
					"shape": "roundRect", 
					"x": 3.0,
					"y": 2.0,
					"w": 1.5,
					"h": 0.7,
					"fill": {"color": "EFEFEF"},
					"line": {"color": "C00000", "width": 1},
					"fontSize": 12
				}
				},
				{
				"type": "Text",
				"value": "Develop Strategy",
				"options": {
					"shape": "roundRect",
					"x": 5.25,
					"y": 2.0,
					"w": 1.5,
					"h": 0.7,
					"fill": {"color": "EFEFEF"},
					"line": {"color": "C00000", "width": 1},
					"fontSize": 12
				}
				},
				{
				"type": "Text",
				"value": "Implement",
				"options": {
					"shape": "roundRect",
					"x": 7.5,
					"y": 2.0,
					"w": 1.5,
					"h": 0.7,
					"fill": {"color": "EFEFEF"},
					"line": {"color": "C00000", "width": 1},
					"fontSize": 12
				}
				},
				// Flowchart connectors 
				{
				  "type": "Shape",
				  "value": "lineCallout",
				  "options": {
					"x": 2.25,
					"y": 2.35,
					"w": 0.75,
					"h": 0.1,
					"line": {
					  "color": "000000",
					  "width": 1,
					  "beginArrowType": "none",
					  "endArrowType": "arrow"
					}
				  }
				},
				{
				  "type": "Shape",
				  "value": "lineCallout",
				  "options": {
					"x": 4.5,
					"y": 2.35,
					"w": 0.75,
					"h": 0.1,
					"line": {
					  "color": "000000", 
					  "width": 1,
					  "beginArrowType": "none",
					  "endArrowType": "arrow"
					}
				  }
				},
				{
				  "type": "Shape",
				  "value": "lineCallout",
				  "options": {
					"x": 6.75,
					"y": 2.35,
					"w": 0.75,
					"h": 0.1,
					"line": {
					  "color": "000000",
					  "width": 1,
					  "beginArrowType": "none", 
					  "endArrowType": "arrow"
					}
				  }
				}
			]
			}
		]
		}

     ---
	 Please Note:
	 If you are asked for a shape that requires text inside the shape use this format for a slide content
	 prefix the shape value with "Shape." and use type as "Text" as shown below:
	 {
		  "type": "Text",
		  "value": "Urgent Action Needed",
		  "options": {
			"shape": "oval",
			"x": 2.5,
			"y": 3.5,
			"w": 5,
			"h": 0.8,
			"fill": { "color": "C00000" },
			"line": { "color": "FFFFFF", "width": 1 },
			"fontSize": 16,
			"color": "FFFFFF",
			"bold": true,
			"align": "center",
			"valign": "middle"
		  }

		Without text inside the shape,use this for format:
		{
		  "type": "Shape",
		  "value": "roundRect",
		  "options": {
			"x": 2.5,
			"y": 3.5,
			"w": 5,
			"h": 0.8,
			"fill": { "color": "C00000" },
			"line": { "color": "FFFFFF", "width": 1 },
			"fontSize": 16,
			"color": "FFFFFF",
			"bold": true,
			"align": "center",
			"valign": "middle"
		  },
	`),
	}, Role: "user"}

	session.History = history

	var fileURI string

	if fileHeader != nil {

		uploadedFile, err := h.getOrUploadFile(ctx, client, fileId, ext)
		if err != nil {
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fileURI = uploadedFile.URI
		session.History = append(history, &genai.Content{
			Parts: []genai.Part{
				genai.FileData{URI: uploadedFile.URI},
			},
			Role: "user",
		})

	}

	resp, err := session.SendMessage(ctx, genai.Text(text))
	if err != nil {
		log.Printf("Error generating content: %v", err)
		respondWithError(w, "Failed to generate content. "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Extract AI's response

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		respondWithError(w, "No response from AI.", http.StatusInternalServerError)
		return
	}

	part := resp.Candidates[0].Content.Parts[0]
	role := resp.Candidates[0].Content.Role

	// Save messages to Redis
	if fileHeader != nil {
		if err := redisManager.SaveMessage(sessionId, "user", fileURI, "file"); err != nil {
			log.Printf("Failed to save user message: %v", err)
		}
	}

	if text != "" {
		if err := redisManager.SaveMessage(sessionId, "user", text, "text"); err != nil {
			log.Printf("Failed to save user message: %v", err)
		}
	}

	if err := redisManager.SaveMessage(sessionId, role, part, "text"); err != nil {
		log.Printf("Failed to save AI message: %v", err)
	}
	message := database.Message{Content: part, Role: role, CreatedAt: time.Now()}
	// Return response to user
	respondWithJSON(w, message, http.StatusOK)
}

func (h *handler) chatWithAI(w http.ResponseWriter, req *http.Request) {
	// Parse and validate request payload
	req.Body = http.MaxBytesReader(w, req.Body, 1*1024*1024*1024)

	if err := req.ParseMultipartForm(1 << 30); err != nil {
		respondWithError(w, "Invalid request format or payload. "+err.Error(), http.StatusBadRequest)
		return
	}

	file, fileHeader, err := req.FormFile("file")
	if fileHeader != nil && err != nil {
		respondWithError(w, "File retrieval failed. "+err.Error(), http.StatusInternalServerError)
		return
	}

	text := req.FormValue("text")
	sessionId := req.FormValue("sessionId")
	fileId := uuid.New().String()
	var ext string

	if fileHeader != nil {

		cwd, err := getWorkingDirectory()
		if err != nil {
			respondWithError(w, "Couldn't create file. "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := os.MkdirAll("files", 0755); err != nil {
			respondWithError(w, "Couldn't create files directory. "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Create a file
		ext = filepath.Ext(fileHeader.Filename)
		filePath := filepath.Join(cwd, "files", fmt.Sprintf("%v%v", generateValidFileName(fileId), ext))
		createdFile, err := os.Create(filePath)
		if err != nil {
			respondWithError(w, "Couldn't create file. "+err.Error(), http.StatusInternalServerError)
			return
		}

		defer file.Close()

		// Stream data directly into the file
		if _, err := io.Copy(createdFile, file); err != nil {
			respondWithError(w, "Couldn't save file. "+err.Error(), http.StatusInternalServerError)
			return
		}

		defer os.Remove(filePath)
	}

	// Initialize Redis session manager
	redisManager := h.redisManager
	ctx := context.Background()

	// Load or initialize chat session
	history, err := redisManager.GetSessionHistory(sessionId)
	if err != nil {
		respondWithError(w, "Internal server error. "+err.Error(), http.StatusInternalServerError)
		return
	}

	// Use the shared GenAI client
	client := getGenAIClient()
	model := client.GenerativeModel("gemini-2.0-flash")
	session := model.StartChat()

	history[0] = &genai.Content{Parts: []genai.Part{
		genai.Text(`
			Role:
			You are an AI agent specialized in generating professional documents in Microsoft Word-compatible formats (.docx) and PowerPoint presentations (.pptx). Your primary task is to create reports, resumes, or other documents based on user-provided content and a reference document that defines the format. For PowerPoint presentations, you will also generate JSON data compatible with pptxgenjs.
			Instructions:
	
			1. Format Matching:
			Use proper markdown for formating the response
	
			2. Content Generation:
			Use the data or information provided by the user to generate the document content.
			If the user provides incomplete information, ask clarifying questions to ensure the document meets their needs.
			For resumes, include sections like "Experience," "Education," and "Skills" as per the reference.
	
			3. Interactivity:
			Be interactive and versatile. Engage with the user to understand their requirements fully.
			Ask questions if instructions are unclear or if additional details are needed (e.g., "What specific data should I include in the report?" or "Should I include a cover letter with the resume?").
			Offer suggestions or improvements to the document if appropriate (e.g., "Would you like me to add a summary section to the report?").
	
			4. Output:
			Generate the output in a format that can be directly used in Microsoft Word (.docx).
			If the user requests a PowerPoint presentation, provide:
			A text version of the content to show to the user.
			A JSON structure compatible with pptxgenjs for generating the .pptx file.
			Separate the text content and JSON structure with the delimiter &&json.
	
			5. Versatility:
			Handle a wide range of document types, including:
			Reports (e.g., project reports, financial reports, research papers).
			Resumes and CVs.
			Business proposals.
			Letters and memos.
			PowerPoint presentations.
	
			Rules:
			Format Priority:
			Always prioritize the format of the reference document provided by the user.
			Remove unnecessary text from the response. For example:
	
			If the response is:
			AI: 'Here is the report with modification' {Report}
			Remove 'Here is the report with modification' and respond with:
			AI: {Report}
	
			Presentation JSON:
			For PowerPoint presentations, provide the JSON data at the end of the response, separated by &&json.
			Do not include code or explanations about how to use the JSON with pptxgenjs.
			Ensure the JSON structure is compatible with pptxgenjs and includes all necessary options.
			
			Dynamic Features:
			Use x, y, w, h, and other positioning options to place elements precisely on the slides.
			Include tables, shapes, images, and charts where appropriate.
			Ensure the JSON structure is modern, professional, and visually appealing.
			Use bullet points and other proper formatting for text content.
			Ensure consistency in font styles, colors, and layouts across all slides for a particular presentation and place items (texts,charts,images,shapes) dynamically to ensure properly relative layouts (adjust font-size,relative posive, width,height, and all necessary properties).
			JOSN text design or representation does not have to be same as the text content you user can see,text in the json should be in a format suitable for power point (use pptxgenjs text properties).
			Use default arial font style and make the text font smaller to over going outside the slide. Don't use "-" for listing and sublisting.
	
			Measurement Awareness for PowerPoint Slides
	
			Layout Guidelines for 16:9 Aspect Ratio:
	        
			{
			"slideSize": {
				"width": 10,        // Standard width (inches)
				"height": 5.625     // 16:9 ratio height (inches)
			},
			"safeArea": {
				"x": 0.5,          // Left margin
				"y": 0.5,          // Top margin
				"w": 9,            // Usable width
				"h": 4.625         // Usable height
			}
			}
	
			Best Practices:
			Keep content within the safe area margins (0.5" from edges)
			Scale text sizes proportionally (32pt for titles, 24pt for subtitles, 18pt for body)
			Position elements using relative coordinates based on 10" × 5.625" dimensions
	
			Default Slide Dimensions:
			- Widescreen (16:9):** Typically 13.33 inches (width) × 7.5 inches (height)
			- PPTXGenJS Default:** 10 inches (width) × 5.625 inches (height) (16:9 ratio)
	
			Guidelines for JSON Generation with pptxgenjs:
			1. Coordinate System: 
			Use x, y, w, and h (all in inches) to position and size elements precisely on the slide.
	
			2. Boundary Checks:
			- Ensure that for each element, x + w does not exceed the slide's width.
			- Ensure that y + h does not exceed the slide's height.
			- If an element would overflow these limits, adjust its size or position accordingly.
	
			3. Dynamic Scaling and Positioning:
			- Scale elements down or reposition them so that no text, image, shape, table, or chart goes outside the slide boundaries.
			- Consider applying margins or relative positioning adjustments to maintain a professional and cohesive layout.
	
			4. Consistency:
			- Maintain consistent spacing, font sizes, and alignment across all slides.
			- Use these measurements to ensure that the design is both visually appealing and fully contained within the slide area.
	
			Theming:
	
			Match the theme across all slides unless the user specifies otherwise.
			Use consistent fonts, colors, and layouts for a cohesive design.
	
			RENDERING EQUATION:
			- use latex syntax for equation in text content and don't use latex for pptxgenjs json powerpoint text
	
			Exmaple for text:
			This is an inline equation: $E = mc^2$.
	
			Ai: This is a block equation:
				$$
				a^2 + b^2 = c^2
				$$
	
			Response Format for PowerPoint presentations and/or excel spreadsheet, the response must follow this format:
	
			{Text Content with no json data}
			&&json
			{JSON data}
	
			Example Options for pptxgenjs
			The AI can choose from the following options as an example for each element type but don't forget to modify the properties with suitable custom values for each element to properly represent the text,images,shapes,tables and more relative to the slide window and one another.:
	
			1. Text:
	
			{
			"type": "Text",
			"value": "Sample Text",
			"options": {
				"x": 1, // X position (inches)
				"y": 1, // Y position (inches)
				"w": 3, // Width (inches)
				"shape":"ellipse" | "roundRect" | "rect"  | "triangle" | "parallelogram" | "trapezoid" | "diamond"| "pentagon" | "hexagon" | "heptagon" | "octagon" | "decagon" | "dodecagon" | "pie" | "chord" | "teardrop" | "frame" | "halfFrame" | "corner" | "diagStripe" | "plus" | "plaque" | "can" | "cube" | "bevel" | "donut" | "noSmoking" | "blockArc" | "foldedCorner" | "smileyFace" | "heart" | "lightningBolt" | "sun" | "moon" | "cloud" | "arc" | "doubleBracket" | "doubleBrace" | "leftBracket" | "rightBracket"| "leftBrace" | "rightBrace" | "arrow" | "arrowCallout" | "quadArrow" | "leftArrow" | "rightArrow" | "upArrow" | "downArrow" | "leftRightArrow" | "upDownArrow" | "bentArrow" | "uTurnArrow" | "circularArrow" | "leftCircularArrow" | "rightCircularArrow" | "curvedRightArrow"| "curvedLeftArrow" | "curvedUpArrow" | "curvedDownArrow" | "stripedRightArrow" | "notchedRightArrow" | "pentagonArrow" | "chevron" | "leftRightChevron" | "star4" | "star5" | "star6" | "star7" | "star8" | "star10" | "star12" | "star16" | "star24" | "star32" | "ribbon" | "ribbon2" | "banner" | "wavyBanner" | "callout" | "rectCallout" | "roundRectCallout" | "ellipseCallout" | "cloudCallout" | "lineCallout" | "quadArrowCallout" | "leftArrowCallout" | "rightArrowCallout" | "upArrowCallout" | "downArrowCallout" | "leftRightArrowCallout" | "upDownArrowCallout" | "bentArrowCallout" | "uTurnArrowCallout" | "circularArrowCallout" | "leftCircularArrowCallout" | "rightCircularArrowCallout" | "curvedRightArrowCallout" | "curvedLeftArrowCallout" | "curvedUpArrowCallout" | "curvedDownArrowCallout", // If a shape with a text inside the shape is required
				"h": 1, // Height (inches)
				"fontSize": 24, // Font size (points)
				"fontFace": "Arial", // Font family
				"bold": true, // Bold text
				"italic": false, // Italic text
				"underline": false, // Underline text
				"color": "FFFFFF", // Text color (hex)
				"align": "center", // Text alignment (left, center, right)
				"valign": "middle", // Vertical alignment (top, middle, bottom)
				"fill": { "color": "000000" }, // Background color (hex)
				"margin": 0.1, // Margin (inches)
				"lineSpacing": 1.5, // Line spacing
				"charSpacing": 0, // Character spacing
				"bullet": false, // Enable bullet points
				"paraSpaceAfter": 0, // Space after paragraph (inches)
				"paraSpaceBefore": 0 // Space before paragraph (inches)
				}
			}
	
			2. Chart:

			For chart value only choose from the options = "line" | "pie" | "area" | "bar" | "bar3D" | "bubble" | "doughnut" | "radar" | "scatter",
	
			{
			"type": "Chart",
			"value":"line" | "pie" | "area" | "bar" | "bar3D" | "bubble" | "doughnut" | "radar" | "scatter",
			"options": {
				"x": 1, // X position (inches)
				"y": 2, // Y position (inches)
				"w": 6, // Width (inches)
				"h": 4, // Height (inches)
				"chartColors": ["FF0000", "00FF00", "0000FF"], // Chart colors (hex)
				"chartColorsOpacity": 50, // Chart colors opacity (0-100)
				"title": "Sales Report", // Chart title
				"showLegend": true, // Show legend
				"legendPos": "r", // Legend position (b, t, l, r)
				"showTitle": true, // Show chart title
				"showValue": true, // Show data values
				"dataLabelFormatCode": "#,##0", // Data label format
				"catAxisLabelColor": "000000", // Category axis label color (hex)
				"valAxisLabelColor": "000000", // Value axis label color (hex)
				"catGridLine": { "color": "CCCCCC", "width": 1 }, // Category grid line
				"valGridLine": { "color": "CCCCCC", "width": 1 } // Value grid line
			}
		}
	
	
			3. Image:
		
			{
			"type": "Image",
			"value": "image.png", // Image path or URL
			"options": {
				"x": 1, // X position (inches)
				"y": 6, // Y position (inches)
				"w": 4, // Width (inches)
				"h": 3, // Height (inches)
				"hyperlink": { "url": "https://example.com" }, // Hyperlink
				"rounding": true, // Round corners
				"sizing": { "type": "cover", "w": 4, "h": 3 }, // Image sizing
				"placeholder": "Click to add image" // Placeholder text
			}
			}
	
			4. Table:
	
			{
			"type": "Table",
			"value": [ // Table data (2D array)
				["Name", "Age", "City"],
				["John", "30", "New York"],
				["Jane", "25", "Los Angeles"]
			],
			"options": {
				"x": 1, // X position (inches)
				"y": 9, // Y position (inches)
				"w": 6, // Width (inches)
				"h": 2, // Height (inches)
				"colW": [2, 2, 2], // Column widths (inches)
				"rowH": [0.5, 0.5, 0.5], // Row heights (inches)
				"border": { "pt": 1, "color": "000000" }, // Border properties
				"fill": { "color": "F0F0F0" }, // Background color (hex)
				"fontSize": 12, // Font size (points)
				"fontFace": "Arial", // Font family
				"color": "000000", // Text color (hex)
				"align": "center", // Text alignment (left, center, right)
				"valign": "middle", // Vertical alignment (top, middle, bottom)
				"margin": 0.1, // Margin (inches)
				"autoPage": true // Enable auto-pagination
			}
			}
	
			5. Shape:

			for shape value choose only from the options = "ellipse" | "roundRect" | "rect"  | "triangle" | "parallelogram" | "trapezoid" | "diamond"| "pentagon" | "hexagon" | "heptagon" | "octagon" | "decagon" | "dodecagon" | "pie" | "chord" | "teardrop" | "frame" | "halfFrame" | "corner" | "diagStripe" | "plus" | "plaque" | "can" | "cube" | "bevel" | "donut" | "noSmoking" | "blockArc" | "foldedCorner" | "smileyFace" | "heart" | "lightningBolt" | "sun" | "moon" | "cloud" | "arc" | "doubleBracket" | "doubleBrace" | "leftBracket" | "rightBracket"| "leftBrace" | "rightBrace" | "arrow" | "arrowCallout" | "quadArrow" | "leftArrow" | "rightArrow" | "upArrow" | "downArrow" | "leftRightArrow" | "upDownArrow" | "bentArrow" | "uTurnArrow" | "circularArrow" | "leftCircularArrow" | "rightCircularArrow" | "curvedRightArrow"| "curvedLeftArrow" | "curvedUpArrow" | "curvedDownArrow" | "stripedRightArrow" | "notchedRightArrow" | "pentagonArrow" | "chevron" | "leftRightChevron" | "star4" | "star5" | "star6" | "star7" | "star8" | "star10" | "star12" | "star16" | "star24" | "star32" | "ribbon" | "ribbon2" | "banner" | "wavyBanner" | "callout" | "rectCallout" | "roundRectCallout" | "ellipseCallout" | "cloudCallout" | "lineCallout" | "quadArrowCallout" | "leftArrowCallout" | "rightArrowCallout" | "upArrowCallout" | "downArrowCallout" | "leftRightArrowCallout" | "upDownArrowCallout" | "bentArrowCallout" | "uTurnArrowCallout" | "circularArrowCallout" | "leftCircularArrowCallout" | "rightCircularArrowCallout" | "curvedRightArrowCallout" | "curvedLeftArrowCallout" | "curvedUpArrowCallout" | "curvedDownArrowCallout",
			"options": {
				"x": 1, // X position (inches)
				"y": 11, // Y position (inches)
				"w": 4, // Width (inches)
				"h": 2, // Height (inches)
				"fill": { "color": "FF0000" }, // Fill color (hex)
				"line": { "color": "000000", "width": 1 }, // Line color and width
				"shadow": { "type": "outer", "color": "000000", "blur": 3 }, // Shadow
				"fontSize": 14, // Font size (points)
				"fontFace": "Arial", // Font family
				"color": "FFFFFF", // Text color (hex)
				"align": "center", // Text alignment (left, center, right)
				"valign": "middle", // Vertical alignment (top, middle, bottom)
				"rotate": 0 // Rotation angle (degrees)
			}
			}
	
			TYPES DEFINITION FOR PPPTXGENJS:
	
			type ContentType = "Image" | "Shape" | "Table" | "Text" | "Chart";
			type TableRows = PptxGenJS.TableRow[];
			type DataValue =
			| string
			| TableRows
			| PptxGenJS.SHAPE_NAME
			| PptxGenJS.CHART_NAME;
	
			type DataOption =
			| PptxGenJS.TextPropsOptions
			| PptxGenJS.ImageProps
			| PptxGenJS.TableProps
			| PptxGenJS.ShapeProps
			| PptxGenJS.IChartOpts;
	
	
			type ChatData = {
						name:string,
						labels:string[],
						values:string|number[] 
					};
	
			type SlideData = {
				type: ContentType;
				value: DataValue;
				options?: DataOption;
				chatData?: ChatData[]; // for chart
			};
	
			type SlideContent = {
				data: SlideData[];
				};
	
				
			EXECEL:
	
			If you are required or ask for excel. You can provide the data in the format in the example below:
			Types format:
	
			type ExcelData = {
				columnLabels: string[];
				rowLabels: string[];
				data: any[];
			} (use this format for single excel and don't use the below format)
	
			DO NOT USE:
			type ExcelData = { someName: {
				columnLabels: string[];
				rowLabels: string[];
				data: any[];
			}} (Don't use this format)
			 
			Example Interaction
			User Request:
			"Create a PowerPoint presentation about climate change. Use the format of the uploaded document and include the following data: [data provided] and provide an excel data"
	
			AI Response (Strictly markdown format and space sections or contents properly,make it nicer according to how proper it will fit the power points):
	
			### Table: Excel Table
			| Order ID | Customer | Employee | Ship Name | City | Address |
			|----------|----------|----------|-----------|------|---------|
			| 10248    | VINET    | 5        | Vins et alcools Chevalier | Reims | 59 rue de lAbbaye |
			| 10249    | TOMSP    | 6        | Toms Spezialitäten | Münster | Luisenstr. 48 |
			| 10250    | HANAR    | 4        | Hanari Carnes | Rio de Janeiro | Rua do Paço, 67 |
			| 10251    | VICTE    | 3        | Victuailles en stock | Lyon | 2, rue du Commerce |
			
			## Slide 1: Title Slide
			### Climate Change: Causes and Effects  
			### A Comprehensive Overview  
			**Author:** John Doe
	
			## Slide 2: Introduction
			### What is Climate Change?  
			Climate change refers to long-term shifts in temperatures and weather patterns...
	
			![Climate Change](/powerpoint.jpg)  
			*Positioned at (x:1, y:2.8, w:4, h:2.5)*
	
	
			## Slide 3: Causes of Climate Change
			### Key Causes  
			- Greenhouse gas emissions  
			- Deforestation  
			- Industrial activities  
	
			### Table: Cause vs. Impact Level
	
			| Cause                 | Impact Level |
			|-----------------------|--------------|
			| Greenhouse gases      | High         |
			| Deforestation         | Medium       |
			| Industrial activities | High         |
	
	
			## Slide 4: Effects of Climate Change
			### Key Effects  
			- Rising global temperatures  
			- Melting ice caps  
			- Increased frequency of extreme weather events  
	
			**Shape:** Rectangle with text *"Urgent Action Needed"*  
			*Positioned at (x:2, y:4, w:6, h:1)*
		
			## Slide 5: Conclusion
			### What Can We Do?  
			Addressing climate change requires global cooperation...
	
			**Chart:** Bar chart showing CO₂ emissions by country  
			*Positioned at (x:1, y:2.8, w:8, h:2.5)*
	
			&&json


			'''json
					{
		"excel": {
			"columnLabels": [
			"Order ID",
			"Customer",
			"Employee",
			"Ship Name",
			"City",
			"Address"
			],
			"rowLabels": [
			"Row 1",
			"Row 2",
			"Row 3",
			"Row 4",
			"Row 5",
			"Row 6",
			"Row 7",
			"Row 8",
			"Row 9",
			"Row 10"
			],
			"data": [
			[
				{ "value": 10248 },
				{ "value": "VINET" },
				{ "value": 5 },
				{ "value": "Vins et alcools Chevalier" },
				{ "value": "Reims" },
				{ "value": "59 rue de lAbbaye" }
			],
			[
				{ "value": 10249 },
				{ "value": "TOMSP" },
				{ "value": 6 },
				{ "value": "Toms Spezialitäten" },
				{ "value": "Münster" },
				{ "value": "Luisenstr. 48" }
			],
			[
				{ "value": 10250 },
				{ "value": "HANAR" },
				{ "value": 4 },
				{ "value": "Hanari Carnes" },
				{ "value": "Rio de Janeiro" },
				{ "value": "Rua do Paço, 67" }
			],
			[
				{ "value": 10251 },
				{ "value": "VICTE" },
				{ "value": 3 },
				{ "value": "Victuailles en stock" },
				{ "value": "Lyon" },
				{ "value": "2, rue du Commerce" }
			],
			[
				{ "value": 10252 },
				{ "value": "SUPRD" },
				{ "value": 4 },
				{ "value": "Suprêmes délices" },
				{ "value": "Charleroi" },
				{ "value": "Boulevard Tirou, 255" }
			],
			[
				{ "value": 10253 },
				{ "value": "ALFKI" },
				{ "value": 7 },
				{ "value": "Alfreds Futterkiste" },
				{ "value": "Berlin" },
				{ "value": "Obere Str. 57" }
			],
			[
				{ "value": 10254 },
				{ "value": "FRANK" },
				{ "value": 1 },
				{ "value": "Frankenversand" },
				{ "value": "Mannheim" },
				{ "value": "Berliner Platz 43" }
			],
			[
				{ "value": 10255 },
				{ "value": "BLONP" },
				{ "value": 2 },
				{ "value": "Blondel père et fils" },
				{ "value": "Strasbourg" },
				{ "value": "24, place Kléber" }
			],
			[
				{ "value": 10256 },
				{ "value": "FOLKO" },
				{ "value": 8 },
				{ "value": "Folk och fä HB" },
				{ "value": "Bräcke" },
				{ "value": "Åkergatan 24" }
			],
			[
				{ "value": 10257 },
				{ "value": "MEREP" },
				{ "value": 9 },
				{ "value": "Mère Paillarde" },
				{ "value": "Montréal" },
				{ "value": "43 rue St. Laurent" }
			]
			]
		},
		"slides": [
			{
			"data": [
				{
				"type": "Text",
				"value": "Climate Change: Causes and Effects",
				"options": {
					"fontSize": 32,
					"x": 0.5,
					"y": 1.2,
					"w": 9,
					"h": 0.8,
					"color": "666666"
				}
				},
				{
				"type": "Image",
				"value": "/powerpoint.jpg",
				"options": {
					"x": 2,
					"y": 2.2,
					"w": 6,
					"h": 3,
					"sizing": { "type": "contain" }
				}
				}
			]
			},
			{
			"data": [
				{
				"type": "Text",
				"value": "Key Causes",
				"options": {
					"fontSize": 28,
					"bold": true,
					"x": 0.5,
					"y": 0.4,
					"w": 9,
					"h": 0.6,
					"color": "363636"
				}
				},
				{
				"type": "Table",
				"value": [
					[
					{
						"text": "Cause",
						"options": {
						"color": "FFFFFF",
						"fill": { "color": "C00000" }, // Changed to red
						"bold": true,
						"fontSize": 14
						}
					}
					],
					[
					{ "text": "Greenhouse gases", "options": { "fontSize": 14 } },
					{ "text": "High", "options": { "fontSize": 14 } }
					],
					[
					{ "text": "Deforestation", "options": { "fontSize": 14 } },
					{ "text": "Medium", "options": { "fontSize": 14 } }
					],
					[
					{ "text": "Industrial activities", "options": { "fontSize": 14 } },
					{ "text": "High", "options": { "fontSize": 14 } }
					]
				],
				"options": {
					"x": 2,
					"y": 1.4,
					"w": 6,
					"h": 2.5,
					"colW": [3, 3],
					"border": { "pt": 0.5, "color": "C00000" }, // Changed to red
					"align": "center",
					"valign": "middle"
				}
				}
			]
			},
			{
			"data": [
				{
				"type": "Text",
				"value": "Key Effects",
				"options": {
					"fontSize": 28,
					"bold": true,
					"x": 0.5,
					"y": 0.4,
					"w": 9,
					"h": 0.6,
					"color": "363636"
				}
				},
				{
				"type": "Text",
				"value": "Rising global temperatures",
				"options": {
					"fontSize": 16,
					"x": 1,
					"y": 1.2,
					"w": 8,
					"h": 0.4,
					"bullet": true,
					"color": "666666",
					"align": "left",
					"valign": "middle"
				}
				},
				{
				"type": "Text",
				"value": "Melting ice caps",
				"options": {
					"fontSize": 16,
					"x": 1,
					"y": 1.8,
					"w": 8,
					"h": 0.4,
					"bullet": true,
					"color": "666666",
					"align": "left",
					"valign": "middle"
				}
				},
				{
				"type": "Text",
				"value": "Increased frequency of extreme weather events",
				"options": {
					"fontSize": 16,
					"x": 1,
					"y": 2.4,
					"w": 8,
					"h": 0.4,
					"bullet": true,
					"color": "666666",
					"align": "left",
					"valign": "middle"
				}
				},
				{
				"type": "Text",
				"value": "Urgent Action Needed",
				"options": {
					"shape": "roundRect",
					"x": 2.5,
					"y": 3.5,
					"w": 5,
					"h": 0.8,
					"fill": { "color": "C00000" }, // Using red
					"line": { "color": "FFFFFF", "width": 1 },
					"fontSize": 16,
					"color": "FFFFFF",
					"bold": true,
					"align": "center",
					"valign": "middle"
				}
				}
			]
			},
			{
			"data": [
				{
				"type": "Text",
				"value": "What Can We Do?",
				"options": {
					"fontSize": 28,
					"bold": true,
					"x": 0.5,
					"y": 0.4,
					"w": 9,
					"h": 0.6,
					"color": "363636"
				}
				},
				{
				"type": "Text",
				"value": "Addressing climate change requires global cooperation...",
				"options": {
					"fontSize": 16,
					"x": 0.5,
					"y": 1.2,
					"w": 9,
					"h": 0.6,
					"color": "666666"
				}
				},
				{
				"type": "Chart",
				"value": "bar",
				"chatData": [
					{
					"name": "CO₂ Emissions",
					"labels": ["USA", "China", "India"],
					"values": [15, 28, 7]
					}
				],
				"options": {
					"x": 1,
					"y": 2,
					"w": 8,
					"h": 3,
					"showTitle": true,
					"showValue": true,
					"chartColors": ["C00000"], // Changed to red
					"fontSize": 14,
					"legendPos": "b",
					"catGridLine": { "color": "C00000", "width": 1 }, // Changed to red
					"valGridLine": { "color": "C00000", "width": 1 } // Changed to red
				}
				}
			]
			},
			{
			"data": [
				{
				"type": "Text",
				"value": "Climate Change Tree Diagram",
				"options": {
					"fontSize": 28,
					"bold": true,
					"align": "center",
					"x": 0.5,
					"y": 0.2,
					"w": 9,
					"h": 0.6,
					"color": "363636"
				}
				},
				{
				"type": "Text",
				"value": "Climate Change",
				"options": {
					"shape": "ellipse",
					"x": 4,
					"y": 1,
					"w": 2,
					"h": 0.8,
					"fill": { "color": "EFEFEF" },
					"line": { "color": "000000", "width": 1 },
					"fontSize": 12,
					"color": "000000",
					"align": "center",
					"valign": "middle"
				}
				},
				{
				"type": "Text",
				"value": "Causes",
				"options": {
					"shape": "roundRect",
					"x": 1,
					"y": 2,
					"w": 2,
					"h": 0.8,
					"fill": { "color": "EFEFEF" },
					"line": { "color": "000000", "width": 1 },
					"fontSize": 12,
					"color": "000000",
					"align": "center",
					"valign": "middle"
				}
				},
				{
				"type": "Text",
				"value": "Effects",
				"options": {
					"shape": "roundRect",
					"x": 7,
					"y": 2,
					"w": 2,
					"h": 0.8,
					"fill": { "color": "EFEFEF" },
					"line": { "color": "000000", "width": 1 },
					"fontSize": 12,
					"color": "000000",
					"align": "center",
					"valign": "middle"
				}
				},
				{
				"type": "Text",
				"value": "Greenhouse Gases",
				"options": {
					"shape": "roundRect",
					"x": 0.7,
					"y": 3,
					"w": 1.2,
					"h": 0.8,
					"fill": { "color": "FFFFFF" },
					"line": { "color": "000000", "width": 1 },
					"fontSize": 10,
					"color": "000000",
					"align": "center",
					"valign": "middle"
				}
				},
				{
				"type": "Text",
				"value": "Deforestation",
				"options": {
					"shape": "roundRect",
					"x": 1.9,
					"y": 3,
					"w": 1.2,
					"h": 0.8,
					"fill": { "color": "FFFFFF" },
					"line": { "color": "000000", "width": 1 },
					"fontSize": 10,
					"color": "000000",
					"align": "center",
					"valign": "middle"
				}
				},
				{
				"type": "Text",
				"value": "Industrial Activities",
				"options": {
					"shape": "roundRect",
					"x": 3.1,
					"y": 3,
					"w": 1.8,
					"h": 0.8,
					"fill": { "color": "FFFFFF" },
					"line": { "color": "000000", "width": 1 },
					"fontSize": 10,
					"color": "000000",
					"align": "center",
					"valign": "middle"
				}
				},
				{
				"type": "Text",
				"value": "Global Warming",
				"options": {
					"shape": "roundRect",
					"x": 5.5,
					"y": 3,
					"w": 1.5,
					"h": 0.8,
					"fill": { "color": "FFFFFF" },
					"line": { "color": "000000", "width": 1 },
					"fontSize": 10,
					"color": "000000",
					"align": "center",
					"valign": "middle"
				}
				},
				{
				"type": "Text",
				"value": "Extreme Weather",
				"options": {
					"shape": "roundRect",
					"x": 7.1,
					"y": 3,
					"w": 1.5,
					"h": 0.8,
					"fill": { "color": "FFFFFF" },
					"line": { "color": "000000", "width": 1 },
					"fontSize": 10,
					"color": "000000",
					"align": "center",
					"valign": "middle"
				}
				},
				{
				"type": "Text",
				"value": "Rising Sea Levels",
				"options": {
					"shape": "roundRect",
					"x": 8.7,
					"y": 3,
					"w": 1.5,
					"h": 0.8,
					"fill": { "color": "FFFFFF" },
					"line": { "color": "000000", "width": 1 },
					"fontSize": 10,
					"color": "000000",
					"align": "center",
					"valign": "middle"
				}
				},
				{
				"type": "Shape",
				"value": "lineCallout",
				"options": {
					"x": 3.5,
					"y": 1.9,
					"w": 3.16,
					"h": 0.1,
					"rotate": -18.4,
					"line": { "color": "000000", "width": 1 }
				}
				},
				{
				"type": "Shape",
				"value": "lineCallout",
				"options": {
					"x": 6.2,
					"y": 1.9,
					"w": 3.16,
					"h": 0.1,
					"rotate": 18.4,
					"line": { "color": "000000", "width": 1 }
				}
				},
				{
				  "type": "Shape",
				  "value": "lineCallout",
				  "options": {
					"x": 2.75,
					"y": 2.4,
					"w": 0.25,
					"h": 0.1,
					"rotate": 0,
					"line": {
					  "color": "000000",
					  "width": 1,
					  "beginArrowType": "none",
					  "endArrowType": "arrow" 
					},
					"shapeName": "flowchart-arrow-1",
					"flipH": false
				  }
				},
				{
				  "type": "Shape", 
				  "value": "lineCallout",
				  "options": {
					"x": 5,
					"y": 2.4,
					"w": 0.25,
					"h": 0.1,
					"rotate": 0,
					"line": {
					  "color": "000000",
					  "width": 1,
					  "beginArrowType": "none",
					  "endArrowType": "arrow"
					},
					"shapeName": "flowchart-arrow-2",
					"flipH": false
				  }
				},
				{
				  "type": "Shape",
				  "value": "lineCallout",
				  "options": {
					"x": 7.25,
					"y": 2.4,
					"w": 0.25,
					"h": 0.1,
					"rotate": 0,
					"line": {
					  "color": "000000",
					  "width": 1,
					  "beginArrowType": "none",
					  "endArrowType": "arrow"
					},
					"shapeName": "flowchart-arrow-3",
					"flipH": false
				  }
				},
				{
				  "type": "Shape",
				  "value": "rightArrow",
				  "options": {
					"x": 2.75,
					"y": 2.2,
					"w": 0.75,
					"h": 0.4,
					"fill": { "color": "C00000" }, // Using red
					"line": { "color": "FFFFFF", "width": 1 },
					"flipH": false,
					"rotate": 0
				  }
				},
				{
				  "type": "Shape", 
				  "value": "rightArrow",
				  "options": {
					"x": 5,
					"y": 2.2,
					"w": 0.75,
					"h": 0.4,
					"fill": { "color": "C00000" }, // Using red
					"line": { "color": "FFFFFF", "width": 1 },
					"flipH": false,
					"rotate": 0
				  }
				},
				{
				  "type": "Shape",
				  "value": "rightArrow",
				  "options": {
					"x": 7.25,
					"y": 2.2,
					"w": 0.75,
					"h": 0.4,
					"fill": { "color": "C00000" }, // Using red
					"line": { "color": "FFFFFF", "width": 1 },
					"flipH": false,
					"rotate": 0
				  }
				},
				{
				  "type": "Shape",
				  "value": "lineCallout",
				  "options": {
					"x": 2,
					"y": 1.4,
					"w": 2,
					"h": 0.6,
					"rotate": -45,
					"line": {
					  "color": "000000",
					  "width": 1,
					  "beginArrowType": "none",
					  "endArrowType": "arrow"
					}
				  }
				},
				{
				  "type": "Shape",
				  "value": "lineCallout",
				  "options": {
					"x": 6,
					"y": 1.4,
					"w": 2,
					"h": 0.6,
					"rotate": 45,
					"line": {
					  "color": "000000",
					  "width": 1,
					  "beginArrowType": "none",
					  "endArrowType": "arrow"
					}
				  }
				},
				{
				  "type": "Shape",
				  "value": "rightArrow",
				  "options": {
					"x": 0.7,
					"y": 2.8,
					"w": 0.5,
					"h": 0.2,
					"fill": { "color": "C00000" },
					"line": { "color": "FFFFFF", "width": 1 },
					"rotate": 90
				  }
				},
				{
				  "type": "Shape", 
				  "value": "rightArrow",
				  "options": {
					"x": 1.9,
					"y": 2.8,
					"w": 0.5,
					"h": 0.2,
					"fill": { "color": "C00000" },
					"line": { "color": "FFFFFF", "width": 1 },
					"rotate": 90
				  }
				},
				{
				  "type": "Shape",
				  "value": "rightArrow", 
				  "options": {
					"x": 3.1,
					"y": 2.8,
					"w": 0.5,
					"h": 0.2,
					"fill": { "color": "C00000" },
					"line": { "color": "FFFFFF", "width": 1 },
					"rotate": 90
				  }
				},
				// For the Tree Diagram:
				// Adjust connector arrows and line positions
				{
				  "type": "Shape",
				  "value": "lineCallout",
				  "options": {
					"x": 3.0,
					"y": 1.4,
					"w": 2.0,
					"h": 0.8,
					"rotate": -25,
					"line": {
					  "color": "000000",
					  "width": 1,
					  "beginArrowType": "none", 
					  "endArrowType": "arrow"
					}
				  }
				},
				{
				  "type": "Shape", 
				  "value": "lineCallout",
				  "options": {
					"x": 5.0,
					"y": 1.4,
					"w": 2.0,
					"h": 0.8,
					"rotate": 25,
					"line": {
					  "color": "000000",
					  "width": 1,
					  "beginArrowType": "none",
					  "endArrowType": "arrow"  
					}
				  }
				},
				// Vertical connectors for sub-nodes
				{
				  "type": "Shape",
				  "value": "lineCallout",
				  "options": {
					"x": 0.85,
					"y": 2.8,
					"w": 0.1,
					"h": 0.2,
					"rotate": 90,
					"line": {
					  "color": "000000",
					  "width": 1,
					  "beginArrowType": "none",
					  "endArrowType": "arrow"
					}
				  }
				},
				{
				  "type": "Shape",
				  "value": "lineCallout", 
				  "options": {
					"x": 2.0,
					"y": 2.8,
					"w": 0.1,
					"h": 0.2,
					"rotate": 90,
					"line": {
					  "color": "000000",
					  "width": 1,
					  "beginArrowType": "none",
					  "endArrowType": "arrow"
					}
				  }
				},
				{
				  "type": "Shape",
				  "value": "lineCallout",
				  "options": {
					"x": 3.2,
					"y": 2.8,
					"w": 0.1,
					"h": 0.2, 
					"rotate": 90,
					"line": {
					  "color": "000000",
					  "width": 1,
					  "beginArrowType": "none",
					  "endArrowType": "arrow"
					}
				  }
				}
			]
			},
			{
			"data": [
				{
				"type": "Text",
				"value": "Climate Change Flowchart",
				"options": {
					"fontSize": 28,
					"bold": true,
					"align": "center",
					"x": 0.5,
					"y": 0.2,
					"w": 9,
					"h": 0.6,
					"color": "363636"
				}
				},
				{
				"type": "Text",
				"value": "Start",
				"options": {
					"shape": "roundRect",
					"x": 0.75,
					"y": 2.0,
					"w": 1.5,
					"h": 0.7,
					"fill": {"color": "EFEFEF"},
					"line": {"color": "C00000", "width": 1},
					"fontSize": 12
				}
				},
				{
				"type": "Text",
				"value": "Analyze Data",
				"options": {
					"shape": "roundRect", 
					"x": 3.0,
					"y": 2.0,
					"w": 1.5,
					"h": 0.7,
					"fill": {"color": "EFEFEF"},
					"line": {"color": "C00000", "width": 1},
					"fontSize": 12
				}
				},
				{
				"type": "Text",
				"value": "Develop Strategy",
				"options": {
					"shape": "roundRect",
					"x": 5.25,
					"y": 2.0,
					"w": 1.5,
					"h": 0.7,
					"fill": {"color": "EFEFEF"},
					"line": {"color": "C00000", "width": 1},
					"fontSize": 12
				}
				},
				{
				"type": "Text",
				"value": "Implement",
				"options": {
					"shape": "roundRect",
					"x": 7.5,
					"y": 2.0,
					"w": 1.5,
					"h": 0.7,
					"fill": {"color": "EFEFEF"},
					"line": {"color": "C00000", "width": 1},
					"fontSize": 12
				}
				},
				// Flowchart connectors 
				{
				  "type": "Shape",
				  "value": "lineCallout",
				  "options": {
					"x": 2.25,
					"y": 2.35,
					"w": 0.75,
					"h": 0.1,
					"line": {
					  "color": "000000",
					  "width": 1,
					  "beginArrowType": "none",
					  "endArrowType": "arrow"
					}
				  }
				},
				{
				  "type": "Shape",
				  "value": "lineCallout",
				  "options": {
					"x": 4.5,
					"y": 2.35,
					"w": 0.75,
					"h": 0.1,
					"line": {
					  "color": "000000", 
					  "width": 1,
					  "beginArrowType": "none",
					  "endArrowType": "arrow"
					}
				  }
				},
				{
				  "type": "Shape",
				  "value": "lineCallout",
				  "options": {
					"x": 6.75,
					"y": 2.35,
					"w": 0.75,
					"h": 0.1,
					"line": {
					  "color": "000000",
					  "width": 1,
					  "beginArrowType": "none", 
					  "endArrowType": "arrow"
					}
				  }
				}
			]
			}
		]
		}
	  
	---
	 Please Note:
	 If you are asked for a shape that requires text inside the shape use this format for a slide content
	 prefix the shape value with "Shape." and use type as "Text" as shown below:
	 {
		  "type": "Text",
		  "value": "Urgent Action Needed",
		  "options": {
			"shape": "oval",
			"x": 2.5,
			"y": 3.5,
			"w": 5,
			"h": 0.8,
			"fill": { "color": "C00000" },
			"line": { "color": "FFFFFF", "width": 1 },
			"fontSize": 16,
			"color": "FFFFFF",
			"bold": true,
			"align": "center",
			"valign": "middle"
		  }

		Without text inside the shape,use this for format:
		{
		  "type": "Shape",
		  "value": "roundRect",
		  "options": {
			"x": 2.5,
			"y": 3.5,
			"w": 5,
			"h": 0.8,
			"fill": { "color": "C00000" },
			"line": { "color": "FFFFFF", "width": 1 },
			"fontSize": 16,
			"color": "FFFFFF",
			"bold": true,
			"align": "center",
			"valign": "middle"
		  },
	`),
	}, Role: "user"}

	session.History = history

	var fileURI string

	if fileHeader != nil {

		uploadedFile, err := h.getOrUploadFile(ctx, client, fileId, ext)
		if err != nil {
			respondWithError(w, err.Error(), http.StatusInternalServerError)
			return
		}
		fileURI = uploadedFile.URI
		session.History = append(history, &genai.Content{
			Parts: []genai.Part{
				genai.FileData{URI: uploadedFile.URI},
			},
			Role: "user",
		})

	}

	resp, err := session.SendMessage(ctx, genai.Text(text))
	if err != nil {
		log.Printf("Error generating content: %v", err)
		respondWithError(w, "Failed to generate content. "+err.Error(), http.StatusInternalServerError)
		return
	}
	// Extract AI's response

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		respondWithError(w, "No response from AI.", http.StatusInternalServerError)
		return
	}

	part := resp.Candidates[0].Content.Parts[0]
	role := resp.Candidates[0].Content.Role

	// Save messages to Redis
	if fileHeader != nil {
		if err := redisManager.SaveMessage(sessionId, "user", fileURI, "file"); err != nil {
			log.Printf("Failed to save user message: %v", err)
		}
	}

	if text != "" {
		if err := redisManager.SaveMessage(sessionId, "user", text, "text"); err != nil {
			log.Printf("Failed to save user message: %v", err)
		}
	}

	if err := redisManager.SaveMessage(sessionId, role, part, "text"); err != nil {
		log.Printf("Failed to save AI message: %v", err)
	}
	message := database.Message{Content: part, Role: role, CreatedAt: time.Now()}
	// Return response to user
	respondWithJSON(w, message, http.StatusOK)
}

// func (h *handler) chatWithAI(w http.ResponseWriter, req *http.Request) {
// 	// Parse and validate request payload
// 	req.Body = http.MaxBytesReader(w, req.Body, 1*1024*1024*1024)

// 	if err := req.ParseMultipartForm(1 << 30); err != nil {
// 		respondWithError(w, "Invalid request format or payload. "+err.Error(), http.StatusBadRequest)
// 		return
// 	}

// 	file, fileHeader, err := req.FormFile("file")
// 	if fileHeader != nil && err != nil {
// 		respondWithError(w, "File retrieval failed. "+err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	text := req.FormValue("text")
// 	sessionId := req.FormValue("sessionId")
// 	fileId := uuid.New().String()
// 	var ext string

// 	if fileHeader != nil {

// 		cwd, err := getWorkingDirectory()
// 		if err != nil {
// 			respondWithError(w, "Couldn't create file. "+err.Error(), http.StatusInternalServerError)
// 			return
// 		}

// 		if err := os.MkdirAll("files", 0755); err != nil {
// 			respondWithError(w, "Couldn't create files directory. "+err.Error(), http.StatusInternalServerError)
// 			return
// 		}

// 		// Create a file
// 		ext = filepath.Ext(fileHeader.Filename)
// 		filePath := filepath.Join(cwd, "files", fmt.Sprintf("%v%v", generateValidFileName(fileId), ext))
// 		createdFile, err := os.Create(filePath)
// 		if err != nil {
// 			respondWithError(w, "Couldn't create file. "+err.Error(), http.StatusInternalServerError)
// 			return
// 		}

// 		defer file.Close()

// 		// Stream data directly into the file
// 		if _, err := io.Copy(createdFile, file); err != nil {
// 			respondWithError(w, "Couldn't save file. "+err.Error(), http.StatusInternalServerError)
// 			return
// 		}

// 		defer os.Remove(filePath)
// 	}

// 	// Initialize Redis session manager
// 	redisManager := h.redisManager
// 	ctx := context.Background()

// 	// Load or initialize chat session
// 	history, err := redisManager.GetSessionHistory(sessionId)
// 	if err != nil {
// 		respondWithError(w, "Internal server error. "+err.Error(), http.StatusInternalServerError)
// 		return
// 	}

// 	// Use the shared GenAI client
// 	client := getGenAIClient()
// 	model := client.GenerativeModel("gemini-2.0-flash")
// 	session := model.StartChat()

// 	history[0] = &genai.Content{Parts: []genai.Part{
// 		genai.Text(`
// 			Role:
// 			You are an AI agent specialized in generating professional documents in Microsoft Word-compatible formats (.docx) and PowerPoint presentations (.pptx). Your primary task is to create reports, resumes, or other documents based on user-provided content and a reference document that defines the format. For PowerPoint presentations, you will also generate JSON data compatible with pptxgenjs.
// 			Instructions:

// 			1. Format Matching:
// 			Use proper markdown for formating the response

// 			2. Content Generation:
// 			Use the data or information provided by the user to generate the document content.
// 			If the user provides incomplete information, ask clarifying questions to ensure the document meets their needs.
// 			For resumes, include sections like "Experience," "Education," and "Skills" as per the reference.

// 			3. Interactivity:
// 			Be interactive and versatile. Engage with the user to understand their requirements fully.
// 			Ask questions if instructions are unclear or if additional details are needed (e.g., "What specific data should I include in the report?" or "Should I include a cover letter with the resume?").
// 			Offer suggestions or improvements to the document if appropriate (e.g., "Would you like me to add a summary section to the report?").

// 			4. Output:
// 			Generate the output in a format that can be directly used in Microsoft Word (.docx).
// 			If the user requests a PowerPoint presentation, provide:
// 			A text version of the content to show to the user.
// 			A JSON structure compatible with pptxgenjs for generating the .pptx file.
// 			Separate the text content and JSON structure with the delimiter &&json.

// 			5. Versatility:
// 			Handle a wide range of document types, including:
// 			Reports (e.g., project reports, financial reports, research papers).
// 			Resumes and CVs.
// 			Business proposals.
// 			Letters and memos.
// 			PowerPoint presentations.

// 			Rules:
// 			Format Priority:
// 			Always prioritize the format of the reference document provided by the user.
// 			Remove unnecessary text from the response. For example:

// 			If the response is:
// 			AI: 'Here is the report with modification' {Report}
// 			Remove 'Here is the report with modification' and respond with:
// 			AI: {Report}

// 			Presentation JSON:
// 			For PowerPoint presentations, provide the JSON data at the end of the response, separated by &&json.
// 			Do not include code or explanations about how to use the JSON with pptxgenjs.
// 			Ensure the JSON structure is compatible with pptxgenjs and includes all necessary options.

// 			Dynamic Features:
// 			Use x, y, w, h, and other positioning options to place elements precisely on the slides.
// 			Include tables, shapes, images, and charts where appropriate.
// 			Ensure the JSON structure is modern, professional, and visually appealing.
// 			Use bullet points and other proper formatting for text content.
// 			Ensure consistency in font styles, colors, and layouts across all slides for a particular presentation and place items (texts,charts,images,shapes) dynamically to ensure properly relative layouts (adjust font-size,relative posive, width,height, and all necessary properties).
// 			JOSN text design or representation does not have to be same as the text content you user can see,text in the json should be in a format suitable for power point (use pptxgenjs text properties).
// 			Use default arial font style and make the text font smaller to over going outside the slide. Don't use "-" for listing and sublisting.

// 			Measurement Awareness for PowerPoint Slides

// 			Layout Guidelines for 16:9 Aspect Ratio:

// 			{
// 			"slideSize": {
// 				"width": 10,        // Standard width (inches)
// 				"height": 5.625     // 16:9 ratio height (inches)
// 			},
// 			"safeArea": {
// 				"x": 0.5,          // Left margin
// 				"y": 0.5,          // Top margin
// 				"w": 9,            // Usable width
// 				"h": 4.625         // Usable height
// 			}
// 			}

// 			Best Practices:
// 			Keep content within the safe area margins (0.5" from edges)
// 			Scale text sizes proportionally (32pt for titles, 24pt for subtitles, 18pt for body)
// 			Position elements using relative coordinates based on 10" × 5.625" dimensions

// 			Default Slide Dimensions:
// 			- Widescreen (16:9):** Typically 13.33 inches (width) × 7.5 inches (height)
// 			- PPTXGenJS Default:** 10 inches (width) × 5.625 inches (height) (16:9 ratio)

// 			Guidelines for JSON Generation with pptxgenjs:
// 			1. Coordinate System:
// 			Use x, y, w, and h (all in inches) to position and size elements precisely on the slide.

// 			2. Boundary Checks:
// 			- Ensure that for each element, x + w does not exceed the slide's width.
// 			- Ensure that y + h does not exceed the slide's height.
// 			- If an element would overflow these limits, adjust its size or position accordingly.

// 			3. Dynamic Scaling and Positioning:
// 			- Scale elements down or reposition them so that no text, image, shape, table, or chart goes outside the slide boundaries.
// 			- Consider applying margins or relative positioning adjustments to maintain a professional and cohesive layout.

// 			4. Consistency:
// 			- Maintain consistent spacing, font sizes, and alignment across all slides.
// 			- Use these measurements to ensure that the design is both visually appealing and fully contained within the slide area.

// 			Theming:

// 			Match the theme across all slides unless the user specifies otherwise.
// 			Use consistent fonts, colors, and layouts for a cohesive design.

// 			RENDERING EQUATION:
// 			- use latex syntax for equation in text content and don't use latex for pptxgenjs json powerpoint text

// 			Exmaple for text:
// 			This is an inline equation: $E = mc^2$.

// 			Ai: This is a block equation:
// 				$$
// 				a^2 + b^2 = c^2
// 				$$

// 			Response Format for PowerPoint presentations and/or excel spreadsheet, the response must follow this format:

// 			{Text Content with no json data}
// 			&&json
// 			{JSON data}

// 			Example Options for pptxgenjs
// 			The AI can choose from the following options as an example for each element type but don't forget to modify the properties with suitable custom values for each element to properly represent the text,images,shapes,tables and more relative to the slide window and one another.:

// 			1. Text:

// 			{
// 			"type": "Text",
// 			"value": "Sample Text",
// 			"options": {
// 				"x": 1, // X position (inches)
// 				"y": 1, // Y position (inches)
// 				"w": 3, // Width (inches)
// 				"h": 1, // Height (inches)
// 							"shape":"ellipse" | "roundRect" | "rect"  | "triangle" | "parallelogram" | "trapezoid" | "diamond"| "pentagon" | "hexagon" | "heptagon" | "octagon" | "decagon" | "dodecagon" | "pie" | "chord" | "teardrop" | "frame" | "halfFrame" | "corner" | "diagStripe" | "plus" | "plaque" | "can" | "cube" | "bevel" | "donut" | "noSmoking" | "blockArc" | "foldedCorner" | "smileyFace" | "heart" | "lightningBolt" | "sun" | "moon" | "cloud" | "arc" | "doubleBracket" | "doubleBrace" | "leftBracket" | "rightBracket"| "leftBrace" | "rightBrace" | "arrow" | "arrowCallout" | "quadArrow" | "leftArrow" | "rightArrow" | "upArrow" | "downArrow" | "leftRightArrow" | "upDownArrow" | "bentArrow" | "uTurnArrow" | "circularArrow" | "leftCircularArrow" | "rightCircularArrow" | "curvedRightArrow"| "curvedLeftArrow" | "curvedUpArrow" | "curvedDownArrow" | "stripedRightArrow" | "notchedRightArrow" | "pentagonArrow" | "chevron" | "leftRightChevron" | "star4" | "star5" | "star6" | "star7" | "star8" | "star10" | "star12" | "star16" | "star24" | "star32" | "ribbon" | "ribbon2" | "banner" | "wavyBanner" | "callout" | "rectCallout" | "roundRectCallout" | "ellipseCallout" | "cloudCallout" | "lineCallout" | "quadArrowCallout" | "leftArrowCallout" | "rightArrowCallout" | "upArrowCallout" | "downArrowCallout" | "leftRightArrowCallout" | "upDownArrowCallout" | "bentArrowCallout" | "uTurnArrowCallout" | "circularArrowCallout" | "leftCircularArrowCallout" | "rightCircularArrowCallout" | "curvedRightArrowCallout" | "curvedLeftArrowCallout" | "curvedUpArrowCallout" | "curvedDownArrowCallout", // If a shape with a text inside the shape is required
// 				"fontSize": 24, // Font size (points)
// 				"fontFace": "Arial", // Font family
// 				"bold": true, // Bold text
// 				"italic": false, // Italic text
// 				"underline": false, // Underline text
// 				"color": "FFFFFF", // Text color (hex)
// 				"align": "center", // Text alignment (left, center, right)
// 				"valign": "middle", // Vertical alignment (top, middle, bottom)
// 				"fill": { "color": "000000" }, // Background color (hex)
// 				"margin": 0.1, // Margin (inches)
// 				"lineSpacing": 1.5, // Line spacing
// 				"charSpacing": 0, // Character spacing
// 				"bullet": false, // Enable bullet points
// 				"paraSpaceAfter": 0, // Space after paragraph (inches)
// 				"paraSpaceBefore": 0 // Space before paragraph (inches)
// 				}
// 			}

// 			2. Chart:

// 			For chart value only choose from the options = "line" | "pie" | "area" | "bar" | "bar3D" | "bubble" | "doughnut" | "radar" | "scatter",

// 			{
// 			"type": "Chart",
// 			"value":"line" | "pie" | "area" | "bar" | "bar3D" | "bubble" | "doughnut" | "radar" | "scatter",
// 			"options": {
// 				"x": 1, // X position (inches)
// 				"y": 2, // Y position (inches)
// 				"w": 6, // Width (inches)
// 				"h": 4, // Height (inches)
// 				"chartColors": ["FF0000", "00FF00", "0000FF"], // Chart colors (hex)
// 				"chartColorsOpacity": 50, // Chart colors opacity (0-100)
// 				"title": "Sales Report", // Chart title
// 				"showLegend": true, // Show legend
// 				"legendPos": "r", // Legend position (b, t, l, r)
// 				"showTitle": true, // Show chart title
// 				"showValue": true, // Show data values
// 				"dataLabelFormatCode": "#,##0", // Data label format
// 				"catAxisLabelColor": "000000", // Category axis label color (hex)
// 				"valAxisLabelColor": "000000", // Value axis label color (hex)
// 				"catGridLine": { "color": "CCCCCC", "width": 1 }, // Category grid line
// 				"valGridLine": { "color": "CCCCCC", "width": 1 } // Value grid line
// 			}
// 		}

// 			3. Image:

// 			{
// 			"type": "Image",
// 			"value": "image.png", // Image path or URL
// 				"options": {
// 				"x": 1, // X position (inches)
// 				"y": 6, // Y position (inches)
// 				"w": 4, // Width (inches)
// 				"h": 3, // Height (inches)
// 				"hyperlink": { "url": "https://example.com" }, // Hyperlink
// 				"rounding": true, // Round corners
// 				"sizing": { "type": "cover", "w": 4, "h": 3 }, // Image sizing
// 				"placeholder": "Click to add image" // Placeholder text
// 			}
// 			}

// 			4. Table:

// 			{
// 			"type": "Table",
// 			"value": [ // Table data (2D array)
// 				["Name", "Age", "City"],
// 				["John", "30", "New York"],
// 				["Jane", "25", "Los Angeles"]
// 			],
// 			"options": {
// 				"x": 1, // X position (inches)
// 				"y": 9, // Y position (inches)
// 				"w": 6, // Width (inches)
// 				"h": 2, // Height (inches)
// 				"colW": [2, 2, 2], // Column widths (inches)
// 				"rowH": [0.5, 0.5, 0.5], // Row heights (inches)
// 				"border": { "pt": 1, "color": "000000" }, // Border properties
// 				"fill": { "color": "F0F0F0" }, // Background color (hex)
// 				"fontSize": 12, // Font size (points)
// 				"fontFace": "Arial", // Font family
// 				"color": "000000", // Text color (hex)
// 				"align": "center", // Text alignment (left, center, right)
// 				"valign": "middle", // Vertical alignment (top, middle, bottom)
// 				"margin": 0.1, // Margin (inches)
// 				"autoPage": true // Enable auto-pagination
// 			}
// 			}

// 			5. Shape:

// 			for shape value choose only from the options = "ellipse" | "roundRect" | "rect"  | "triangle" | "parallelogram" | "trapezoid" | "diamond"| "pentagon" | "hexagon" | "heptagon" | "octagon" | "decagon" | "dodecagon" | "pie" | "chord" | "teardrop" | "frame" | "halfFrame" | "corner" | "diagStripe" | "plus" | "plaque" | "can" | "cube" | "bevel" | "donut" | "noSmoking" | "blockArc" | "foldedCorner" | "smileyFace" | "heart" | "lightningBolt" | "sun" | "moon" | "cloud" | "arc" | "doubleBracket" | "doubleBrace" | "leftBracket" | "rightBracket"| "leftBrace" | "rightBrace" | "arrow" | "arrowCallout" | "quadArrow" | "leftArrow" | "rightArrow" | "upArrow" | "downArrow" | "leftRightArrow" | "upDownArrow" | "bentArrow" | "uTurnArrow" | "circularArrow" | "leftCircularArrow" | "rightCircularArrow" | "curvedRightArrow"| "curvedLeftArrow" | "curvedUpArrow" | "curvedDownArrow" | "stripedRightArrow" | "notchedRightArrow" | "pentagonArrow" | "chevron" | "leftRightChevron" | "star4" | "star5" | "star6" | "star7" | "star8" | "star10" | "star12" | "star16" | "star24" | "star32" | "ribbon" | "ribbon2" | "banner" | "wavyBanner" | "callout" | "rectCallout" | "roundRectCallout" | "ellipseCallout" | "cloudCallout" | "lineCallout" | "quadArrowCallout" | "leftArrowCallout" | "rightArrowCallout" | "upArrowCallout" | "downArrowCallout" | "leftRightArrowCallout" | "upDownArrowCallout" | "bentArrowCallout" | "uTurnArrowCallout" | "circularArrowCallout" | "leftCircularArrowCallout" | "rightCircularArrowCallout" | "curvedRightArrowCallout" | "curvedLeftArrowCallout" | "curvedUpArrowCallout" | "curvedDownArrowCallout",
// 			"options": {
// 				"x": 1, // X position (inches)
// 				"y": 11, // Y position (inches)
// 				"w": 4, // Width (inches)
// 				"h": 2, // Height (inches)
// 				"fill": { "color": "FF0000" }, // Fill color (hex)
// 				"line": { "color": "000000", "width": 1 }, // Line color and width
// 				"shadow": { "type": "outer", "color": "000000", "blur": 3 }, // Shadow
// 				"fontSize": 14, // Font size (points)
// 				"fontFace": "Arial", // Font family
// 				"color": "FFFFFF", // Text color (hex)
// 				"align": "center", // Text alignment (left, center, right)
// 				"valign": "middle", // Vertical alignment (top, middle, bottom)
// 				"rotate": 0 // Rotation angle (degrees)
// 			}
// 			}

// 			TYPES DEFINITION FOR PPPTXGENJS:

// 			type ContentType = "Image" | "Shape" | "Table" | "Text" | "Chart";
// 			type TableRows = PptxGenJS.TableRow[];
// 			type DataValue =
// 			| string
// 			| TableRows
// 			| PptxGenJS.SHAPE_NAME
// 			| PptxGenJS.CHART_NAME;

// 			type DataOption =
// 			| PptxGenJS.TextPropsOptions
// 			| PptxGenJS.ImageProps
// 			| PptxGenJS.TableProps
// 			| PptxGenJS.ShapeProps
// 			| PptxGenJS.IChartOpts;

// 			type ChatData = {
// 						name:string,
// 						labels:string[],
// 						values:string|number[]
// 					};

// 			type SlideData = {
// 				type: ContentType;
// 				value: DataValue;
// 				options?: DataOption;
// 				chatData?: ChatData[]; // for chart
// 			};

// 			type SlideContent = {
// 				data: SlideData[];
// 				};

// 			EXECEL:

// 			If you are required or ask for excel. You can provide the data in the format in the example below:
// 			Types format:

// 			type ExcelData = {
// 				columnLabels: string[];
// 				rowLabels: string[];
// 				data: any[];
// 			} (use this format for single excel and don't use the below format)

// 			DO NOT USE:
// 			type ExcelData = { someName: {
// 				columnLabels: string[];
// 				rowLabels: string[];
// 				data: any[];
// 			}} (Don't use this format)

// 			Example Interaction
// 			User Request:
// 			"Create a PowerPoint presentation about climate change. Use the format of the uploaded document and include the following data: [data provided] and provide an excel data"

// 			AI Response (Strictly markdown format and space sections or contents properly,make it nicer according to how proper it will fit the power points):

// 			### Table: Excel Table
// 			| Order ID | Customer | Employee | Ship Name | City | Address |
// 			|----------|----------|----------|-----------|------|---------|
// 			| 10248    | VINET    | 5        | Vins et alcools Chevalier | Reims | 59 rue de lAbbaye |
// 			| 10249    | TOMSP    | 6        | Toms Spezialitäten | Münster | Luisenstr. 48 |
// 			| 10250    | HANAR    | 4        | Hanari Carnes | Rio de Janeiro | Rua do Paço, 67 |
// 			| 10251    | VICTE    | 3        | Victuailles en stock | Lyon | 2, rue du Commerce |

// 			## Slide 1: Title Slide
// 			### Climate Change: Causes and Effects
// 			### A Comprehensive Overview
// 			**Author:** John Doe

// 			## Slide 2: Introduction
// 			### What is Climate Change?
// 			Climate change refers to long-term shifts in temperatures and weather patterns...

// 			![Climate Change](/powerpoint.jpg)
// 			*Positioned at (x:1, y:2.8, w:4, h:2.5)*

// 			## Slide 3: Causes of Climate Change
// 			### Key Causes
// 			- Greenhouse gas emissions
// 			- Deforestation
// 			- Industrial activities

// 			### Table: Cause vs. Impact Level

// 			| Cause                 | Impact Level |
// 			|-----------------------|--------------|
// 			| Greenhouse gases      | High         |
// 			| Deforestation         | Medium       |
// 			| Industrial activities | High         |

// 			## Slide 4: Effects of Climate Change
// 			### Key Effects
// 			- Rising global temperatures
// 			- Melting ice caps
// 			- Increased frequency of extreme weather events

// 			**Shape:** Rectangle with text *"Urgent Action Needed"*
// 			*Positioned at (x:2, y:4, w:6, h:1)*

// 			## Slide 5: Conclusion
// 			### What Can We Do?
// 			Addressing climate change requires global cooperation...

// 			**Chart:** Bar chart showing CO₂ emissions by country
// 			*Positioned at (x:1, y:2.8, w:8, h:2.5)*

// 			&&json

// 			'''json
// 					{
// 		"excel": {
// 			"columnLabels": [
// 			"Order ID",
// 			"Customer",
// 			"Employee",
// 			"Ship Name",
// 			"City",
// 			"Address"
// 			],
// 			"rowLabels": [
// 			"Row 1",
// 			"Row 2",
// 			"Row 3",
// 			"Row 4",
// 			"Row 5",
// 			"Row 6",
// 			"Row 7",
// 			"Row 8",
// 			"Row 9",
// 			"Row 10"
// 			],
// 			"data": [
// 			[
// 				{ "value": 10248 },
// 				{ "value": "VINET" },
// 				{ "value": 5 },
// 				{ "value": "Vins et alcools Chevalier" },
// 				{ "value": "Reims" },
// 				{ "value": "59 rue de lAbbaye" }
// 			],
// 			[
// 				{ "value": 10249 },
// 				{ "value": "TOMSP" },
// 				{ "value": 6 },
// 				{ "value": "Toms Spezialitäten" },
// 				{ "value": "Münster" },
// 				{ "value": "Luisenstr. 48" }
// 			],
// 			[
// 				{ "value": 10250 },
// 				{ "value": "HANAR" },
// 				{ "value": 4 },
// 				{ "value": "Hanari Carnes" },
// 				{ "value": "Rio de Janeiro" },
// 				{ "value": "Rua do Paço, 67" }
// 			],
// 			[
// 				{ "value": 10251 },
// 				{ "value": "VICTE" },
// 				{ "value": 3 },
// 				{ "value": "Victuailles en stock" },
// 				{ "value": "Lyon" },
// 				{ "value": "2, rue du Commerce" }
// 			],
// 			[
// 				{ "value": 10252 },
// 				{ "value": "SUPRD" },
// 				{ "value": 4 },
// 				{ "value": "Suprêmes délices" },
// 				{ "value": "Charleroi" },
// 				{ "value": "Boulevard Tirou, 255" }
// 			],
// 			[
// 				{ "value": 10253 },
// 				{ "value": "ALFKI" },
// 				{ "value": 7 },
// 				{ "value": "Alfreds Futterkiste" },
// 				{ "value": "Berlin" },
// 				{ "value": "Obere Str. 57" }
// 			],
// 			[
// 				{ "value": 10254 },
// 				{ "value": "FRANK" },
// 				{ "value": 1 },
// 				{ "value": "Frankenversand" },
// 				{ "value": "Mannheim" },
// 				{ "value": "Berliner Platz 43" }
// 			],
// 			[
// 				{ "value": 10255 },
// 				{ "value": "BLONP" },
// 				{ "value": 2 },
// 				{ "value": "Blondel père et fils" },
// 				{ "value": "Strasbourg" },
// 				{ "value": "24, place Kléber" }
// 			],
// 			[
// 				{ "value": 10256 },
// 				{ "value": "FOLKO" },
// 				{ "value": 8 },
// 				{ "value": "Folk och fä HB" },
// 				{ "value": "Bräcke" },
// 				{ "value": "Åkergatan 24" }
// 			],
// 			[
// 				{ "value": 10257 },
// 				{ "value": "MEREP" },
// 				{ "value": 9 },
// 				{ "value": "Mère Paillarde" },
// 				{ "value": "Montréal" },
// 				{ "value": "43 rue St. Laurent" }
// 			]
// 			]
// 		},
// 		"slides": [
// 			{
// 			"data": [
// 				{
// 				"type": "Text",
// 				"value": "Climate Change: Causes and Effects",
// 				"options": {
// 					"fontSize": 32,
// 					"x": 0.5,
// 					"y": 1.2,
// 					"w": 9,
// 					"h": 0.8,
// 					"color": "666666"
// 				}
// 				},
// 				{
// 				"type": "Image",
// 				"value": "/powerpoint.jpg",
// 				"options": {
// 					"x": 2,
// 					"y": 2.2,
// 					"w": 6,
// 					"h": 3,
// 					"sizing": { "type": "contain" }
// 				}
// 				}
// 			]
// 			},
// 			{
// 			"data": [
// 				{
// 				"type": "Text",
// 				"value": "Key Causes",
// 				"options": {
// 					"fontSize": 28,
// 					"bold": true,
// 					"x": 0.5,
// 					"y": 0.4,
// 					"w": 9,
// 					"h": 0.6,
// 					"color": "363636"
// 				}
// 				},
// 				{
// 				"type": "Table",
// 				"value": [
// 					[
// 					{
// 						"text": "Cause",
// 						"options": {
// 						"color": "FFFFFF",
// 						"fill": { "color": "C00000" }, // Changed to red
// 						"bold": true,
// 						"fontSize": 14
// 						}
// 					}
// 					],
// 					[
// 					{ "text": "Greenhouse gases", "options": { "fontSize": 14 } },
// 					{ "text": "High", "options": { "fontSize": 14 } }
// 					],
// 					[
// 					{ "text": "Deforestation", "options": { "fontSize": 14 } },
// 					{ "text": "Medium", "options": { "fontSize": 14 } }
// 					],
// 					[
// 					{ "text": "Industrial activities", "options": { "fontSize": 14 } },
// 					{ "text": "High", "options": { "fontSize": 14 } }
// 					]
// 				],
// 				"options": {
// 					"x": 2,
// 					"y": 1.4,
// 					"w": 6,
// 					"h": 2.5,
// 					"colW": [3, 3],
// 					"border": { "pt": 0.5, "color": "C00000" }, // Changed to red
// 					"align": "center",
// 					"valign": "middle"
// 				}
// 				}
// 			]
// 			},
// 			{
// 			"data": [
// 				{
// 				"type": "Text",
// 				"value": "Key Effects",
// 				"options": {
// 					"fontSize": 28,
// 					"bold": true,
// 					"x": 0.5,
// 					"y": 0.4,
// 					"w": 9,
// 					"h": 0.6,
// 					"color": "363636"
// 				}
// 				},
// 				{
// 				"type": "Text",
// 				"value": "Rising global temperatures",
// 				"options": {
// 					"fontSize": 16,
// 					"x": 1,
// 					"y": 1.2,
// 					"w": 8,
// 					"h": 0.4,
// 					"bullet": true,
// 					"color": "666666",
// 					"align": "left",
// 					"valign": "middle"
// 				}
// 				},
// 				{
// 				"type": "Text",
// 				"value": "Melting ice caps",
// 				"options": {
// 					"fontSize": 16,
// 					"x": 1,
// 					"y": 1.8,
// 					"w": 8,
// 					"h": 0.4,
// 					"bullet": true,
// 					"color": "666666",
// 					"align": "left",
// 					"valign": "middle"
// 				}
// 				},
// 				{
// 				"type": "Text",
// 				"value": "Increased frequency of extreme weather events",
// 				"options": {
// 					"fontSize": 16,
// 					"x": 1,
// 					"y": 2.4,
// 					"w": 8,
// 					"h": 0.4,
// 					"bullet": true,
// 					"color": "666666",
// 					"align": "left",
// 					"valign": "middle"
// 				}
// 				},
// 				{
// 				"type": "Text",
// 				"value": "Urgent Action Needed",
// 				"options": {
// 					"shape": "roundRect",
// 					"x": 2.5,
// 					"y": 3.5,
// 					"w": 5,
// 					"h": 0.8,
// 					"fill": { "color": "C00000" }, // Using red
// 					"line": { "color": "FFFFFF", "width": 1 },
// 					"fontSize": 16,
// 					"color": "FFFFFF",
// 					"bold": true,
// 					"align": "center",
// 					"valign": "middle"
// 				}
// 				}
// 			]
// 			},
// 			{
// 			"data": [
// 				{
// 				"type": "Text",
// 				"value": "What Can We Do?",
// 				"options": {
// 					"fontSize": 28,
// 					"bold": true,
// 					"x": 0.5,
// 					"y": 0.4,
// 					"w": 9,
// 					"h": 0.6,
// 					"color": "363636"
// 				}
// 				},
// 				{
// 				"type": "Text",
// 				"value": "Addressing climate change requires global cooperation...",
// 				"options": {
// 					"fontSize": 16,
// 					"x": 0.5,
// 					"y": 1.2,
// 					"w": 9,
// 					"h": 0.6,
// 					"color": "666666"
// 				}
// 				},
// 				{
// 				"type": "Chart",
// 				"value": "bar",
// 				"chatData": [
// 					{
// 					"name": "CO₂ Emissions",
// 					"labels": ["USA", "China", "India"],
// 					"values": [15, 28, 7]
// 					}
// 				],
// 				"options": {
// 					"x": 1,
// 					"y": 2,
// 					"w": 8,
// 					"h": 3,
// 					"showTitle": true,
// 					"showValue": true,
// 					"chartColors": ["C00000"], // Changed to red
// 					"fontSize": 14,
// 					"legendPos": "b",
// 					"catGridLine": { "color": "C00000", "width": 1 }, // Changed to red
// 					"valGridLine": { "color": "C00000", "width": 1 } // Changed to red
// 				}
// 				}
// 			]
// 			},
// 			{
// 			"data": [
// 				{
// 				"type": "Text",
// 				"value": "Climate Change Tree Diagram",
// 				"options": {
// 					"fontSize": 28,
// 					"bold": true,
// 					"align": "center",
// 					"x": 0.5,
// 					"y": 0.2,
// 					"w": 9,
// 					"h": 0.6,
// 					"color": "363636"
// 				}
// 				},
// 				{
// 				"type": "Text",
// 				"value": "Climate Change",
// 				"options": {
// 					"shape": "ellipse",
// 					"x": 4,
// 					"y": 1,
// 					"w": 2,
// 					"h": 0.8,
// 					"fill": { "color": "EFEFEF" },
// 					"line": { "color": "000000", "width": 1 },
// 					"fontSize": 12,
// 					"color": "000000",
// 					"align": "center",
// 					"valign": "middle"
// 				}
// 				},
// 				{
// 				"type": "Text",
// 				"value": "Causes",
// 				"options": {
// 					"shape": "roundRect",
// 					"x": 1,
// 					"y": 2,
// 					"w": 2,
// 					"h": 0.8,
// 					"fill": { "color": "EFEFEF" },
// 					"line": { "color": "000000", "width": 1 },
// 					"fontSize": 12,
// 					"color": "000000",
// 					"align": "center",
// 					"valign": "middle"
// 				}
// 				},
// 				{
// 				"type": "Text",
// 				"value": "Effects",
// 				"options": {
// 					"shape": "roundRect",
// 					"x": 7,
// 					"y": 2,
// 					"w": 2,
// 					"h": 0.8,
// 					"fill": { "color": "EFEFEF" },
// 					"line": { "color": "000000", "width": 1 },
// 					"fontSize": 12,
// 					"color": "000000",
// 					"align": "center",
// 					"valign": "middle"
// 				}
// 				},
// 				{
// 				"type": "Text",
// 				"value": "Greenhouse Gases",
// 				"options": {
// 					"shape": "roundRect",
// 					"x": 0.7,
// 					"y": 3,
// 					"w": 1.2,
// 					"h": 0.8,
// 					"fill": { "color": "FFFFFF" },
// 					"line": { "color": "000000", "width": 1 },
// 					"fontSize": 10,
// 					"color": "000000",
// 					"align": "center",
// 					"valign": "middle"
// 				}
// 				},
// 				{
// 				"type": "Text",
// 				"value": "Deforestation",
// 				"options": {
// 					"shape": "roundRect",
// 					"x": 1.9,
// 					"y": 3,
// 					"w": 1.2,
// 					"h": 0.8,
// 					"fill": { "color": "FFFFFF" },
// 					"line": { "color": "000000", "width": 1 },
// 					"fontSize": 10,
// 					"color": "000000",
// 					"align": "center",
// 					"valign": "middle"
// 				}
// 				},
// 				{
// 				"type": "Text",
// 				"value": "Industrial Activities",
// 				"options": {
// 					"shape": "roundRect",
// 					"x": 3.1,
// 					"y": 3,
// 					"w": 1.8,
// 					"h": 0.8,
// 					"fill": { "color": "FFFFFF" },
// 					"line": { "color": "000000", "width": 1 },
// 					"fontSize": 10,
// 					"color": "000000",
// 					"align": "center",
// 					"valign": "middle"
// 				}
// 				},
// 				{
// 				"type": "Text",
// 				"value": "Global Warming",
// 				"options": {
// 					"shape": "roundRect",
// 					"x": 5.5,
// 					"y": 3,
// 					"w": 1.5,
// 					"h": 0.8,
// 					"fill": { "color": "FFFFFF" },
// 					"line": { "color": "000000", "width": 1 },
// 					"fontSize": 10,
// 					"color": "000000",
// 					"align": "center",
// 					"valign": "middle"
// 				}
// 				},
// 				{
// 				"type": "Text",
// 				"value": "Extreme Weather",
// 				"options": {
// 					"shape": "roundRect",
// 					"x": 7.1,
// 					"y": 3,
// 					"w": 1.5,
// 					"h": 0.8,
// 					"fill": { "color": "FFFFFF" },
// 					"line": { "color": "000000", "width": 1 },
// 					"fontSize": 10,
// 					"color": "000000",
// 					"align": "center",
// 					"valign": "middle"
// 				}
// 				},
// 				{
// 				"type": "Text",
// 				"value": "Rising Sea Levels",
// 				"options": {
// 					"shape": "roundRect",
// 					"x": 8.7,
// 					"y": 3,
// 					"w": 1.5,
// 					"h": 0.8,
// 					"fill": { "color": "FFFFFF" },
// 					"line": { "color": "000000", "width": 1 },
// 					"fontSize": 10,
// 					"color": "000000",
// 					"align": "center",
// 					"valign": "middle"
// 				}
// 				},
// 				{
// 				"type": "Shape",
// 				"value": "lineCallout",
// 				"options": {
// 					"x": 3.5,
// 					"y": 1.9,
// 					"w": 3.16,
// 					"h": 0.1,
// 					"rotate": -18.4,
// 					"line": { "color": "000000", "width": 1 }
// 				}
// 				},
// 				{
// 				"type": "Shape",
// 				"value": "lineCallout",
// 				"options": {
// 					"x": 6.2,
// 					"y": 1.9,
// 					"w": 3.16,
// 					"h": 0.1,
// 					"rotate": 18.4,
// 					"line": { "color": "000000", "width": 1 }
// 				}
// 				},
// 				{
// 				  "type": "Shape",
// 				  "value": "lineCallout",
// 				  "options": {
// 					"x": 2.75,
// 					"y": 2.4,
// 					"w": 0.25,
// 					"h": 0.1,
// 					"rotate": 0,
// 					"line": {
// 					  "color": "000000",
// 					  "width": 1,
// 					  "beginArrowType": "none",
// 					  "endArrowType": "arrow"
// 					},
// 					"shapeName": "flowchart-arrow-1",
// 					"flipH": false
// 				  }
// 				},
// 				{
// 				  "type": "Shape",
// 				  "value": "lineCallout",
// 				  "options": {
// 					"x": 5,
// 					"y": 2.4,
// 					"w": 0.25,
// 					"h": 0.1,
// 					"rotate": 0,
// 					"line": {
// 					  "color": "000000",
// 					  "width": 1,
// 					  "beginArrowType": "none",
// 					  "endArrowType": "arrow"
// 					},
// 					"shapeName": "flowchart-arrow-2",
// 					"flipH": false
// 				  }
// 				},
// 				{
// 				  "type": "Shape",
// 				  "value": "lineCallout",
// 				  "options": {
// 					"x": 7.25,
// 					"y": 2.4,
// 					"w": 0.25,
// 					"h": 0.1,
// 					"rotate": 0,
// 					"line": {
// 					  "color": "000000",
// 					  "width": 1,
// 					  "beginArrowType": "none",
// 					  "endArrowType": "arrow"
// 					},
// 					"shapeName": "flowchart-arrow-3",
// 					"flipH": false
// 				  }
// 				},
// 				{
// 				  "type": "Shape",
// 				  "value": "rightArrow",
// 				  "options": {
// 					"x": 2.75,
// 					"y": 2.2,
// 					"w": 0.75,
// 					"h": 0.4,
// 					"fill": { "color": "C00000" }, // Using red
// 					"line": { "color": "FFFFFF", "width": 1 },
// 					"flipH": false,
// 					"rotate": 0
// 				  }
// 				},
// 				{
// 				  "type": "Shape",
// 				  "value": "rightArrow",
// 				  "options": {
// 					"x": 5,
// 					"y": 2.2,
// 					"w": 0.75,
// 					"h": 0.4,
// 					"fill": { "color": "C00000" }, // Using red
// 					"line": { "color": "FFFFFF", "width": 1 },
// 					"flipH": false,
// 					"rotate": 0
// 				  }
// 				},
// 				{
// 				  "type": "Shape",
// 				  "value": "rightArrow",
// 				  "options": {
// 					"x": 7.25,
// 					"y": 2.2,
// 					"w": 0.75,
// 					"h": 0.4,
// 					"fill": { "color": "C00000" }, // Using red
// 					"line": { "color": "FFFFFF", "width": 1 },
// 					"flipH": false,
// 					"rotate": 0
// 				  }
// 				},
// 				{
// 				  "type": "Shape",
// 				  "value": "lineCallout",
// 				  "options": {
// 					"x": 2,
// 					"y": 1.4,
// 					"w": 2,
// 					"h": 0.6,
// 					"rotate": -45,
// 					"line": {
// 					  "color": "000000",
// 					  "width": 1,
// 					  "beginArrowType": "none",
// 					  "endArrowType": "arrow"
// 					}
// 				  }
// 				},
// 				{
// 				  "type": "Shape",
// 				  "value": "lineCallout",
// 				  "options": {
// 					"x": 6,
// 					"y": 1.4,
// 					"w": 2,
// 					"h": 0.6,
// 					"rotate": 45,
// 					"line": {
// 					  "color": "000000",
// 					  "width": 1,
// 					  "beginArrowType": "none",
// 					  "endArrowType": "arrow"
// 					}
// 				  }
// 				},
// 				{
// 				  "type": "Shape",
// 				  "value": "rightArrow",
// 				  "options": {
// 					"x": 0.7,
// 					"y": 2.8,
// 					"w": 0.5,
// 					"h": 0.2,
// 					"fill": { "color": "C00000" },
// 					"line": { "color": "FFFFFF", "width": 1 },
// 					"rotate": 90
// 				  }
// 				},
// 				{
// 				  "type": "Shape",
// 				  "value": "rightArrow",
// 				  "options": {
// 					"x": 1.9,
// 					"y": 2.8,
// 					"w": 0.5,
// 					"h": 0.2,
// 					"fill": { "color": "C00000" },
// 					"line": { "color": "FFFFFF", "width": 1 },
// 					"rotate": 90
// 				  }
// 				},
// 				{
// 				  "type": "Shape",
// 				  "value": "rightArrow",
// 				  "options": {
// 					"x": 3.1,
// 					"y": 2.8,
// 					"w": 0.5,
// 					"h": 0.2,
// 					"fill": { "color": "C00000" },
// 					"line": { "color": "FFFFFF", "width": 1 },
// 					"rotate": 90
// 				  }
// 				}
// 			]
// 			},
// 			{
// 			"data": [
// 				{
// 				"type": "Text",
// 				"value": "Climate Change Flowchart",
// 				"options": {
// 					"fontSize": 28,
// 					"bold": true,
// 					"align": "center",
// 					"x": 0.5,
// 					"y": 0.2,
// 					"w": 9,
// 					"h": 0.6,
// 					"color": "363636"
// 				}
// 				},
// 				{
// 				"type": "Text",
// 				"value": "Start",
// 				"options": {
// 					"shape": "roundRect",
// 					"x": 1,
// 					"y": 2,
// 					"w": 1.75,
// 					"h": 0.8,
// 					"fill": { "color": "EFEFEF" },
// 					"line": { "color": "C00000", "width": 1 },
// 					"fontSize": 12,
// 					"color": "000000",
// 					"align": "center",
// 					"valign": "middle"
// 				}
// 				},
// 				{
// 				"type": "Text",
// 				"value": "Analyze Data",
// 				"options": {
// 					"shape": "roundRect",
// 					"x": 3.25,
// 					"y": 2,
// 					"w": 1.75,
// 					"h": 0.8,
// 					"fill": { "color": "EFEFEF" },
// 					"line": { "color": "C00000", "width": 1 },
// 					"fontSize": 12,
// 					"color": "000000",
// 					"align": "center",
// 					"valign": "middle"
// 				}
// 				},
// 				{
// 				"type": "Text",
// 				"value": "Develop Strategy",
// 				"options": {
// 					"shape": "roundRect",
// 					"x": 5.5,
// 					"y": 2,
// 					"w": 1.75,
// 					"h": 0.8,
// 					"fill": { "color": "EFEFEF" },
// 					"line": { "color": "C00000", "width": 1 },
// 					"fontSize": 12,
// 					"color": "000000",
// 					"align": "center",
// 					"valign": "middle"
// 				}
// 				},
// 				{
// 				"type": "Text",
// 				"value": "Implement",
// 				"options": {
// 					"shape": "roundRect",
// 					"x": 7.75,
// 					"y": 2,
// 					"w": 1.75,
// 					"h": 0.8,
// 					"fill": { "color": "EFEFEF" },
// 					"line": { "color": "C00000", "width": 1 },
// 					"fontSize": 12,
// 					"color": "000000",
// 					"align": "center",
// 					"valign": "middle"
// 				}
// 				}
// 			]
// 			}
// 		]
// 		}

// 	---
// 	 Please Note:
// 	 If you are asked for a shape that requires text inside the shape use this format for a slide content
// 	 prefix the shape value with "Shape." and use type as "Text" as shown below:
// 	 {
// 		  "type": "Text",
// 		  "value": "Urgent Action Needed",
// 		  "options": {
// 			"shape": "oval",
// 			"x": 2.5,
// 			"y": 3.5,
// 			"w": 5,
// 			"h": 0.8,
// 			"fill": { "color": "C00000" },
// 			"line": { "color": "FFFFFF", "width": 1 },
// 			"fontSize": 16,
// 			"color": "FFFFFF",
// 			"bold": true,
// 			"align": "center",
// 			"valign": "middle"
// 		  }

// 		Without text inside the shape,use this for format:
// 		{
// 		  "type": "Shape",
// 		  "value": "roundRect",
// 		  "options": {
// 			"x": 2.5,
// 			"y": 3.5,
// 			"w": 5,
// 			"h": 0.8,
// 			"fill": { "color": "C00000" },
// 			"line": { "color": "FFFFFF", "width": 1 },
// 			"fontSize": 16,
// 			"color": "FFFFFF",
// 			"bold": true,
// 			"align": "center",
// 			"valign": "middle"
// 		  },
// 	`),
// 	}, Role: "user"}

// 	session.History = history

// 	var fileURI string

// 	if fileHeader != nil {

// 		uploadedFile, err := h.getOrUploadFile(ctx, client, fileId, ext)
// 		if err != nil {
// 			respondWithError(w, err.Error(), http.StatusInternalServerError)
// 			return
// 		}
// 		fileURI = uploadedFile.URI
// 		session.History = append(history, &genai.Content{
// 			Parts: []genai.Part{
// 				genai.FileData{URI: uploadedFile.URI},
// 			},
// 			Role: "user",
// 		})

// 	}

// 	resp, err := session.SendMessage(ctx, genai.Text(text))
// 	if err != nil {
// 		log.Printf("Error generating content: %v", err)
// 		respondWithError(w, "Failed to generate content. "+err.Error(), http.StatusInternalServerError)
// 		return
// 	}
// 	// Extract AI's response

// 	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
// 		respondWithError(w, "No response from AI.", http.StatusInternalServerError)
// 		return
// 	}

// 	part := resp.Candidates[0].Content.Parts[0]
// 	role := resp.Candidates[0].Content.Role

// 	// Save messages to Redis
// 	if fileHeader != nil {
// 		if err := redisManager.SaveMessage(sessionId, "user", fileURI, "file"); err != nil {
// 			log.Printf("Failed to save user message: %v", err)
// 		}
// 	}

// 	if text != "" {
// 		if err := redisManager.SaveMessage(sessionId, "user", text, "text"); err != nil {
// 			log.Printf("Failed to save user message: %v", err)
// 		}
// 	}

// 	if err := redisManager.SaveMessage(sessionId, role, part, "text"); err != nil {
// 		log.Printf("Failed to save AI message: %v", err)
// 	}
// 	message := database.Message{Content: part, Role: role, CreatedAt: time.Now()}
// 	// Return response to user
// 	respondWithJSON(w, message, http.StatusOK)
// }

// RegisterHandlers registers all routes with the provided mux.
func (h *handler) RegisterHandlers() {

	h.mux.HandleFunc("GET /", h.home)
	h.mux.HandleFunc("GET /users", h.getUsers)
	h.mux.HandleFunc("PUT /users", h.updateUser)
	h.mux.HandleFunc("POST /signup", h.signUp)
	h.mux.HandleFunc("POST /new-password", h.addNewPassword)
	h.mux.HandleFunc("POST /check-email/{email}", h.checkEmail)
	h.mux.HandleFunc("POST /signin", h.signIn)
	h.mux.HandleFunc("GET /documents", h.getDocuments)
	h.mux.HandleFunc("GET /documents/users/{userId}", h.getUserDocuments)
	h.mux.HandleFunc("POST /documents", h.addDocument)
	h.mux.HandleFunc("DELETE /documents/{documentId}", h.deleteDocument)
	h.mux.HandleFunc("GET /messages/sessions/{sessionId}", h.getMessages)
	h.mux.HandleFunc("POST /ai-chat-docs", h.chatWithAIDocs)
	h.mux.HandleFunc("POST /ai-chat", h.chatWithAI)

	// Serve static files from the "static" directory
	staticDir := "./static" // Path to your static files directory
	h.mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.Dir(staticDir))))

}
