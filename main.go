package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "sync"
    "time"
    "image"
    "image/color"
    "image/draw"
    "image/png"
)

type GameState struct {
    BallX    int `json:"ball_x"`
    BallY    int `json:"ball_y"`
    BallDirX int `json:"ball_dir_x"`
    BallDirY int `json:"ball_dir_y"`
    Player1Y int `json:"player1_y"`
    Player2Y int `json:"player2_y"`
}

var gameState = GameState{
    BallX:    300, // Position initiale de la balle
    BallY:    200,
    BallDirX: 1,   // Vitesse et direction de la balle
    BallDirY: 1,
    Player1Y: 100, // Position initiale des raquettes
    Player2Y: 100,
}

var lock sync.Mutex
var gamePaused bool = false

const (
    canvasWidth  = 600
    canvasHeight = 400
    paddleHeight = 100
    paddleWidth  = 10
    ballSize     = 10
)

func main() {

    http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        http.ServeFile(w, r, "index.html")
    })

    http.HandleFunc("/pause", func(w http.ResponseWriter, r *http.Request) {
        lock.Lock()
        defer lock.Unlock()

        // Bascule l'état de pause
        gamePaused = !gamePaused

        // Réponse simple pour indiquer l'état actuel du jeu
        if gamePaused {
            fmt.Fprintln(w, "Game paused")
        } else {
            fmt.Fprintln(w, "Game resumed")
        }
    })
 
    http.HandleFunc("/screen", func(w http.ResponseWriter, r *http.Request) {
        img := drawGameStateToImage()
        w.Header().Set("Content-Type", "image/png")
        if err := png.Encode(w, img); err != nil {
            log.Println("Failed to encode PNG:", err)
            http.Error(w, "Failed to encode PNG", http.StatusInternalServerError)
        }
    })

    http.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
        lock.Lock()
        defer lock.Unlock()
        json.NewEncoder(w).Encode(gameState)
    })

    http.HandleFunc("/cmd", func(w http.ResponseWriter, r *http.Request) {
	lock.Lock()
	defer lock.Unlock()

	var cmd struct {
		Player string `json:"player"`
		PosY   int    `json:"pos_y"` // Utiliser PosY pour définir la position en Y
	}

	if err := json.NewDecoder(r.Body).Decode(&cmd); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// Assurez-vous que la position Y est dans les limites du canvas
	if cmd.PosY < 0 {
		cmd.PosY = 0
	} else if cmd.PosY > canvasHeight-paddleHeight {
		cmd.PosY = canvasHeight - paddleHeight
	}

	// Mettre à jour la position de la raquette en fonction du joueur spécifié
	if cmd.Player == "p1" {
		gameState.Player1Y = cmd.PosY
	} else if cmd.Player == "p2" {
		gameState.Player2Y = cmd.PosY
	}
    })
    go gameLoop()

    fmt.Println("Server started at http://localhost:8080")
    log.Fatal(http.ListenAndServe(":8080", nil))
}

func gameLoop() {
    ticker := time.NewTicker(20 * time.Millisecond)
    for range ticker.C {
        lock.Lock()

        if !gamePaused {
            // Mise à jour de la position de la balle
            gameState.BallX += gameState.BallDirX
            gameState.BallY += gameState.BallDirY

            // AI for Player 1 to follow the ball when it's moving towards Player 1
            if gameState.BallDirX < 0 { // Ball is moving towards Player 1
                aiPaddleSpeedP1 := 2 // Adjust this for difficulty
                if gameState.BallY > gameState.Player1Y+paddleHeight/2 && gameState.Player1Y < canvasHeight-paddleHeight {
                    gameState.Player1Y += aiPaddleSpeedP1
                } else if gameState.BallY < gameState.Player1Y+paddleHeight/2 && gameState.Player1Y > 0 {
                    gameState.Player1Y -= aiPaddleSpeedP1
                }
            }

            // AI for Player 2 to follow the ball when it's moving towards Player 2
            if gameState.BallDirX > 0 { // Ball is moving towards Player 2
                aiPaddleSpeedP2 := 2 // Adjust this for difficulty, can be different from Player 1 for asymmetry
                if gameState.BallY > gameState.Player2Y+paddleHeight/2 && gameState.Player2Y < canvasHeight-paddleHeight {
                    gameState.Player2Y += aiPaddleSpeedP2
                } else if gameState.BallY < gameState.Player2Y+paddleHeight/2 && gameState.Player2Y > 0 {
                    gameState.Player2Y -= aiPaddleSpeedP2
                }
            }
            // Rebond sur les murs haut et bas
            if gameState.BallY <= 0 || gameState.BallY >= canvasHeight-ballSize {
                gameState.BallDirY = -gameState.BallDirY
            }
            // Rebond sur les raquettes ou réinitialisation si la balle touche les bords gauche ou droit
            if gameState.BallX <= paddleWidth {
                if gameState.BallY >= gameState.Player1Y && gameState.BallY <= gameState.Player1Y+paddleHeight {
                    gameState.BallDirX = -gameState.BallDirX
                } else if gameState.BallX <= 0 {
                    resetBall()
                }
            } else if gameState.BallX >= canvasWidth-paddleWidth-ballSize {
                if gameState.BallY >= gameState.Player2Y && gameState.BallY <= gameState.Player2Y+paddleHeight {
                    gameState.BallDirX = -gameState.BallDirX
                } else if gameState.BallX >= canvasWidth-ballSize {
                    resetBall()
                }
            }
        }
	lock.Unlock()
    }
}

func resetBall() {
    gameState.BallX = canvasWidth / 2
    gameState.BallY = canvasHeight / 2
    gameState.BallDirX = -gameState.BallDirX
    gameState.BallDirY = -gameState.BallDirY
}

func drawGameStateToImage() *image.RGBA {
    lock.Lock()
    defer lock.Unlock()

    img := image.NewRGBA(image.Rect(0, 0, canvasWidth, canvasHeight))
    // Set background color
    draw.Draw(img, img.Bounds(), &image.Uniform{color.Black}, image.Point{}, draw.Src)

    // Draw ball
    ballColor := color.RGBA{255, 0, 0, 255} // Red ball
    ballRect := image.Rect(gameState.BallX-ballSize/2, gameState.BallY-ballSize/2, gameState.BallX+ballSize/2, gameState.BallY+ballSize/2)
    draw.Draw(img, ballRect, &image.Uniform{ballColor}, image.Point{}, draw.Src)

    // Draw paddles
    paddleColor := color.RGBA{255, 255, 255, 255} // White paddles
    p1Rect := image.Rect(10, gameState.Player1Y, 10+paddleWidth, gameState.Player1Y+paddleHeight)
    p2Rect := image.Rect(canvasWidth-10-paddleWidth, gameState.Player2Y, canvasWidth-10, gameState.Player2Y+paddleHeight)
    draw.Draw(img, p1Rect, &image.Uniform{paddleColor}, image.Point{}, draw.Src)
    draw.Draw(img, p2Rect, &image.Uniform{paddleColor}, image.Point{}, draw.Src)

    return img
}
