package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"net/http"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/notnil/chess"
)

type OutcomeMessage struct {
	GameID  string `json:"gameId"`
	Outcome string `json:"outcome"`
	Winner  string `json:"winner"`
}

type MoveMessage struct {
	GameID string `json:"gameId"`
	Color  string `json:"color"`
	Move   string `json:"move"`
}

type MoveAnswer struct {
	GameID string `json:"gameId"`
	Fen    string `json:"fen"`
}

type JoinMessage struct {
	GameID string `json:"gameId"`
}

type AgainstMessage struct {
	ID    string `json:"id"`
	Color string `json:"color"`
}

type WebsocketMessage struct {
	Type    string `json:"type"`
	Payload string `json:"payload"`
}

type HelloMessage struct {
	ID string `json:"id"`
}

type PossibleMovesMessage struct {
	GameID string   `json:"gameId"`
	Moves  []string `json:"moves"`
}

type Client struct {
	ID   string
	Conn *websocket.Conn
}

type Game struct {
	WhitePlayerId string `json:"whitePlayerId"`
	BlackPlayerId string `json:"blackPlayerId"`
	Game          *chess.Game
}

type StoredGames map[string]StoredGame

type StoredGame struct {
	PGNStr        string `json:"pgn"`
	WhitePlayerId string `json:"whitePlayerId"`
	BlackPlayerId string `json:"blackPlayerId"`
}

type CapturedPiecesMessage struct {
	GameID string   `json:"gameId"`
	White  []string `json:"white"`
	Black  []string `json:"black"`
}

type CreateGameRequest struct {
	Player1        string `json:"player1"`
	Player2        string `json:"player2"`
	PreferredColor string `json:"preferredColor"`
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func GenerateCapturedPiecesMessage(game *Game, gameId string) ([]byte, error) {
	whiteStart := map[chess.PieceType]int{
		chess.Pawn:   8,
		chess.Rook:   2,
		chess.Knight: 2,
		chess.Bishop: 2,
		chess.Queen:  1,
		chess.King:   1,
	}
	blackStart := map[chess.PieceType]int{
		chess.Pawn:   8,
		chess.Rook:   2,
		chess.Knight: 2,
		chess.Bishop: 2,
		chess.Queen:  1,
		chess.King:   1,
	}
	whiteCurrent := map[chess.PieceType]int{
		chess.Pawn:   0,
		chess.Rook:   0,
		chess.Knight: 0,
		chess.Bishop: 0,
		chess.Queen:  0,
		chess.King:   0,
	}
	blackCurrent := map[chess.PieceType]int{
		chess.Pawn:   0,
		chess.Rook:   0,
		chess.Knight: 0,
		chess.Bishop: 0,
		chess.Queen:  0,
		chess.King:   0,
	}

	board := game.Game.Position().Board()

	// Durch alle Felder gehen
	for sq := chess.A1; sq <= chess.H8; sq++ {
		piece := board.Piece(sq)
		if piece != chess.NoPiece {
			if piece.Color() == chess.White {
				whiteCurrent[piece.Type()]++
			} else {
				blackCurrent[piece.Type()]++
			}
		}
	}

	whiteCaptured := map[chess.PieceType]int{}
	blackCaptured := map[chess.PieceType]int{}

	for pt, startCount := range whiteStart {
		whiteCaptured[pt] = startCount - whiteCurrent[pt]
	}
	for pt, startCount := range blackStart {
		blackCaptured[pt] = startCount - blackCurrent[pt]
	}

	whitePieces := make([]string, 0)
	blackPieces := make([]string, 0)

	for pt, count := range whiteCaptured {
		for i := 0; i < count; i++ {
			whitePieces = append(whitePieces, chess.NewPiece(pt, chess.White).String())
		}
	}

	for pt, count := range blackCaptured {
		for i := 0; i < count; i++ {
			blackPieces = append(blackPieces, chess.NewPiece(pt, chess.Black).String())
		}
	}

	capturedPiecesMsg := CapturedPiecesMessage{
		GameID: gameId,
		White:  whitePieces,
		Black:  blackPieces,
	}

	data, err := json.Marshal(capturedPiecesMsg)
	if err != nil {
		return nil, err
	}

	msg := WebsocketMessage{
		Type:    "capturedPieces",
		Payload: string(data),
	}

	data, err = json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func GenerateOutcomeMessage(game *Game, gameId string) ([]byte, error) {
	outcome := game.Game.Outcome()
	method := game.Game.Method()

	outcomeMsg := OutcomeMessage{
		GameID:  gameId,
		Outcome: "",
		Winner:  "",
	}

	if outcome == chess.WhiteWon {
		outcomeMsg.Outcome = "white won"
		outcomeMsg.Winner = "w"
	}
	if outcome == chess.BlackWon {
		outcomeMsg.Outcome = "black won"
		outcomeMsg.Winner = "b"
	}
	if outcome == chess.Draw {
		outcomeMsg.Outcome = "draw"
	}
	if method == chess.Checkmate {
		if outcomeMsg.Outcome == "" {
			outcomeMsg.Outcome = "checkmate"
		} else {
			outcomeMsg.Outcome += " checkmate"
		}
	}

	data, err := json.Marshal(outcomeMsg)
	if err != nil {
		return nil, err
	}

	msg := WebsocketMessage{
		Type:    "outcome",
		Payload: string(data),
	}

	data, err = json.Marshal(msg)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func GeneratePossibleMovesMessage(game *Game, gameId string, clientId string) ([]byte, error) {
	possibleMoves := make([]string, 0)
	moves := game.Game.ValidMoves()

	if game.Game.Position().Turn() == chess.White && game.WhitePlayerId != clientId {
		possibleMovesMsg := PossibleMovesMessage{
			GameID: gameId,
			Moves:  possibleMoves,
		}

		data, err := json.Marshal(possibleMovesMsg)
		if err != nil {
			return nil, err
		}

		msgPossibleMoves := WebsocketMessage{
			Type:    "possibleMoves",
			Payload: string(data),
		}

		possibleMoveData, err := json.Marshal(msgPossibleMoves)
		if err != nil {
			return nil, err
		}

		return possibleMoveData, nil
	}

	if game.Game.Position().Turn() == chess.Black && game.BlackPlayerId != clientId {
		possibleMovesMsg := PossibleMovesMessage{
			GameID: gameId,
			Moves:  possibleMoves,
		}

		data, err := json.Marshal(possibleMovesMsg)
		if err != nil {
			return nil, err
		}

		msgPossibleMoves := WebsocketMessage{
			Type:    "possibleMoves",
			Payload: string(data),
		}

		possibleMoveData, err := json.Marshal(msgPossibleMoves)
		if err != nil {
			return nil, err
		}

		return possibleMoveData, nil
	}

	for _, move := range moves {
		possibleMoves = append(possibleMoves, move.String())
	}

	possibleMovesMsg := PossibleMovesMessage{
		GameID: gameId,
		Moves:  possibleMoves,
	}

	data, err := json.Marshal(possibleMovesMsg)
	if err != nil {
		return nil, err
	}

	msgPossibleMoves := WebsocketMessage{
		Type:    "possibleMoves",
		Payload: string(data),
	}

	possibleMoveData, err := json.Marshal(msgPossibleMoves)
	if err != nil {
		return nil, err
	}

	return possibleMoveData, nil
}

func GenerateMoveAnswerMessage(game *Game, move MoveMessage) ([]byte, error) {
	answer := MoveAnswer{
		GameID: move.GameID,
		Fen:    game.Game.Position().String(),
	}

	data, err := json.Marshal(answer)
	if err != nil {
		return nil, err
	}

	answerMsg := WebsocketMessage{
		Type:    "move",
		Payload: string(data),
	}

	moveData, err := json.Marshal(answerMsg)
	if err != nil {
		return nil, err
	}

	return moveData, nil
}

func HandleMove(
	wsMsg WebsocketMessage,
	client *Client,
) error {
	var move MoveMessage
	err := json.Unmarshal([]byte(wsMsg.Payload), &move)
	if err != nil {
		return err
	}

	game, ok := games[move.GameID]
	if !ok {
		return errors.New("Game not found")
	}

	err = game.Game.MoveStr(move.Move)
	if err != nil {
		return err
	}

	err = SaveGames()
	if err != nil {
		return err
	}

	if game.Game.Outcome() != chess.NoOutcome {
		moveAnswer, err := GenerateMoveAnswerMessage(game, move)
		if err != nil {
			return err
		}

		err = client.Conn.WriteMessage(websocket.TextMessage, moveAnswer)
		if err != nil {
			return err
		}

		data, err := GenerateOutcomeMessage(game, move.GameID)
		if err != nil {
			return err
		}

		err = client.Conn.WriteMessage(websocket.TextMessage, data)
		if err != nil {
			return err
		}

		return nil
	}

	moveAnswer, err := GenerateMoveAnswerMessage(game, move)
	if err != nil {
		return err
	}

	possibleMoveData, err := GeneratePossibleMovesMessage(game, move.GameID, client.ID)
	if err != nil {
		return err
	}

	capturedPiecesData, err := GenerateCapturedPiecesMessage(game, move.GameID)
	if err != nil {
		return err
	}

	for _, client := range connectedClients {
		err = client.Conn.WriteMessage(websocket.TextMessage, moveAnswer)
		if err != nil {
			fmt.Println(err)
			continue
		}
		err = client.Conn.WriteMessage(websocket.TextMessage, possibleMoveData)
		if err != nil {
			fmt.Println(err)
			continue
		}
		err = client.Conn.WriteMessage(websocket.TextMessage, capturedPiecesData)
		if err != nil {
			fmt.Println(err)
		}
	}

	return nil
}

func GenerateAgainstMessage(game *Game, client *Client) ([]byte, error) {
	againstMsg := AgainstMessage{
		ID:    "",
		Color: "",
	}

	if game.WhitePlayerId == client.ID {
		againstMsg.Color = "b"
		againstMsg.ID = game.BlackPlayerId
	} else if game.BlackPlayerId == client.ID {
		againstMsg.Color = "w"
		againstMsg.ID = game.WhitePlayerId
	} else {
		return nil, errors.New("Player not in game")
	}

	data, err := json.Marshal(againstMsg)
	if err != nil {
		return nil, err
	}

	against := WebsocketMessage{
		Type:    "against",
		Payload: string(data),
	}

	data, err = json.Marshal(against)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func HandleJoin(
	wsMsg WebsocketMessage,
	newClient *Client,
) error {
	var join JoinMessage
	err := json.Unmarshal([]byte(wsMsg.Payload), &join)
	if err != nil {
		return err
	}

	game, ok := games[join.GameID]
	if !ok {
		return errors.New("Game not found")
	}

	data, err := GenerateAgainstMessage(game, newClient)
	if err != nil {
		return err
	}

	err = newClient.Conn.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		return err
	}

	data, err = GeneratePossibleMovesMessage(game, join.GameID, newClient.ID)
	if err != nil {
		return err
	}

	err = newClient.Conn.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		return err
	}

	outcome, err := GenerateOutcomeMessage(game, join.GameID)
	if err != nil {
		return err
	}

	err = newClient.Conn.WriteMessage(websocket.TextMessage, outcome)
	if err != nil {
		return err
	}

	capturedPiecesData, err := GenerateCapturedPiecesMessage(game, join.GameID)
	if err != nil {
		return err
	}

	err = newClient.Conn.WriteMessage(websocket.TextMessage, capturedPiecesData)
	if err != nil {
		return err
	}

	return nil
}

func WsHandler(c *gin.Context, id string) error {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return err
	}

	newClient := &Client{
		ID:   id,
		Conn: conn,
	}

	connectedClients = append(connectedClients, newClient)

	helloMsg := HelloMessage{
		ID: id,
	}

	data, err := json.Marshal(helloMsg)
	if err != nil {
		return err
	}

	hello := WebsocketMessage{
		Type:    "hello",
		Payload: string(data),
	}

	data, err = json.Marshal(hello)
	if err != nil {
		return err
	}

	err = conn.WriteMessage(websocket.TextMessage, data)
	if err != nil {
		return err
	}

	defer func() {
		for i, client := range connectedClients {
			if client.ID == newClient.ID {
				connectedClients = append(connectedClients[:i], connectedClients[i+1:]...)
				break
			}
		}
		conn.Close()
	}()

	for {
		_, msg, err := conn.ReadMessage()
		if err != nil {
			continue
		}

		var wsMsg WebsocketMessage
		err = json.Unmarshal(msg, &wsMsg)
		if err != nil {
			continue
		}

		switch wsMsg.Type {
		case "leave":
			break
		case "join":
			err := HandleJoin(wsMsg, newClient)
			if err != nil {
				fmt.Println(err)
			}
		case "move":
			err := HandleMove(wsMsg, newClient)

			if err != nil {
				fmt.Println(err)
			}
		default:
			break
		}
	}
}

func SaveGames() error {
	stored := make(StoredGames)

	for id, game := range games {
		pgn, err := game.Game.MarshalText()
		if err != nil {
			return err
		}

		storedGame := StoredGame{
			PGNStr:        string(pgn),
			WhitePlayerId: game.WhitePlayerId,
			BlackPlayerId: game.BlackPlayerId,
		}

		stored[id] = storedGame
	}

	data, err := json.Marshal(stored)
	if err != nil {
		return err
	}

	return os.WriteFile("games.json", data, 0644)
}

func LoadGames() error {
	data, err := os.ReadFile("games.json")
	if err != nil {
		return err
	}

	storedGames := make(StoredGames)
	if err := json.Unmarshal(data, &storedGames); err != nil {
		return err
	}

	games = make(map[string]*Game)
	for id, storedGame := range storedGames {
		game := chess.NewGame(chess.UseNotation(chess.UCINotation{}))

		if err := game.UnmarshalText([]byte(storedGame.PGNStr)); err != nil {
			return err
		}

		newGame := &Game{
			Game:          game,
			WhitePlayerId: storedGame.WhitePlayerId,
			BlackPlayerId: storedGame.BlackPlayerId,
		}

		games[id] = newGame
	}

	return nil
}

var games = make(map[string]*Game)
var connectedClients = make([]*Client, 0)

func main() {
	r := gin.Default()
	err := LoadGames()
	if err != nil {
		fmt.Println(err)
	}

	r.GET("/ws", func(c *gin.Context) {
		queryId := c.Query("id")
		var id string

		if queryId == "" {
			id = uuid.New().String()
		} else {
			id = queryId
		}

		err := WsHandler(c, id)
		if err != nil {
			fmt.Println(err)
		}
	})

	r.GET("/game/:id", func(c *gin.Context) {
		id := c.Param("id")
		game, ok := games[id]

		if !ok {
			c.JSON(404, gin.H{"message": "Game not found"})
			return
		}

		fens := make([]string, 0)

		for _, pos := range game.Game.Positions() {
			fens = append(fens, pos.String())
		}

		c.JSON(200, fens)
	})

	r.POST("/game", func(c *gin.Context) {
		id := uuid.New().String()
		var request CreateGameRequest
		err := c.BindJSON(&request)
		if err != nil {
			c.JSON(400, gin.H{"message": "Bad request"})
			return
		}

		fenStr := "rnbqkbnr/pppppppp/8/8/8/8/PPPPPPPP/RNBQKBNR w KQkq - 0 1"
		fen, err := chess.FEN(fenStr)
		if err != nil {
			c.JSON(500, gin.H{"message": "Internal server error"})
			return
		}

		game := chess.NewGame(fen, chess.UseNotation(chess.UCINotation{}))

		newGame := &Game{
			Game:          game,
			WhitePlayerId: "",
			BlackPlayerId: "",
		}

		if request.PreferredColor == "w" {
			newGame.WhitePlayerId = request.Player1
			newGame.BlackPlayerId = request.Player2
		} else if request.PreferredColor == "b" {
			newGame.WhitePlayerId = request.Player2
			newGame.BlackPlayerId = request.Player1
		} else {
			randomNumber := rand.IntN(1-0) + 0
			if randomNumber == 0 {
				newGame.WhitePlayerId = request.Player1
				newGame.BlackPlayerId = request.Player2
			} else {
				newGame.WhitePlayerId = request.Player2
				newGame.BlackPlayerId = request.Player1
			}
		}

		games[id] = newGame

		err = SaveGames()
		if err != nil {
			c.JSON(500, gin.H{"message": "Internal server error"})
			return
		}

		c.JSON(200, gin.H{"id": id})
	})

	err = r.Run(":4000")
	if err != nil {
		fmt.Println(err)
	}
}
