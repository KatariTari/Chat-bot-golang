package main

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/go-telegram-bot-api/telegram-bot-api"
	_"github.com/lib/pq"
)

// Константы для подключения к PostgreSQL и токена бота.
const (
	dbHost     = "localhost"
	dbPort     = "5432"
	dbUser     = "postgres"
	dbPassword = "32"
	dbName     = "lab24b"
	botToken   = "6656614266:AAEG6KVzxnlgXLIobv68PVoK9bCiDKjaN9Y"
)

var db *sql.DB

type TelegramBot struct {
	API                   *tgbotapi.BotAPI        // API телеграмма
	Updates               tgbotapi.UpdatesChannel // Канал обновлений
	ActiveContactRequests []int64                 // ID чатов, от которых мы ожидаем номер
	UserStates            map[int64]string
}

// Инициализация базы данных при запуске программы.
func init() {
	var err error

	// Строка подключения к базе данных.
	dbInfo := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	// Открываем соединение с базой данных.
	db, err = sql.Open("postgres", dbInfo)
	if err != nil {
		log.Fatal(err)
	}

	// Проверяем соединение с базой данных.
	err = db.Ping()
	if err != nil {
		log.Fatal(err)
	}

	log.Println("Successfully connected to the database")

	// Создаем таблицу пользователей, если она еще не существует.
	createTableQuery := `
CREATE TABLE IF NOT EXISTS users (
id SERIAL PRIMARY KEY,
telegram_id INT UNIQUE,
last_name VARCHAR(255),
first_name VARCHAR(255),
course VARCHAR(255),
group_name VARCHAR(255),
password VARCHAR(255)
);
`
	_, err = db.Exec(createTableQuery)
	if err != nil {
		log.Fatal(err)
	}
}

// Основная функция, запускающая бота и обрабатывающая команды пользователя.
func main() {
	telegramBot := TelegramBot{}
	telegramBot.init()
	// Формируем строку подключения
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		dbHost, dbPort, dbUser, dbPassword, dbName)

	// Подключаемся к базе данных
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		fmt.Println("Error opening database:", err)
		return
	}
	defer db.Close()

	// Проверяем соединение
	err = db.Ping()
	if err != nil {
		fmt.Println("Error connecting to the database:", err)
		return
	}

	fmt.Println("Successfully connected to the database!")

}

// Функция инициализации бота.
func (telegramBot *TelegramBot) init() {
	// Инициализация бота.
	botAPI, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}

	telegramBot.API = botAPI

	botAPI.Debug = true

	log.Printf("Authorized on account %s", botAPI.Self.UserName)

	// Создаем канал для получения обновлений от бота.
	botUpdate := tgbotapi.NewUpdate(0) // Инициализация канала обновлений
	botUpdate.Timeout = 64
	botUpdates, err := telegramBot.API.GetUpdatesChan(botUpdate)

	if err != nil {
		log.Fatal(err)
	}
	telegramBot.Updates = botUpdates

	// Обрабатываем полученные обновления.
	for update := range botUpdates {
		if update.Message == nil {
			continue
		}

		// Проверяем, зарегистрирован ли пользователь
		if !telegramBot.isUserRegistered(update.Message.Chat.ID) {
			switch update.Message.Text {
			case "/start":
				// Отправляем клавиатуру "Регистрация"
				telegramBot.sendMainMenu(update.Message.Chat.ID)
			case "Вход":
				telegramBot.sendMainMenu(update.Message.Chat.ID)
			case "Регистрация":
				// Начинаем регистрацию
				telegramBot.registerUser(update.Message.Chat.ID)
			default:
				botAPI.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Unknown command"))
			}
		} else {
			// Если пользователь уже зарегистрирован, отправляем сообщение об этом.
			botAPI.Send(tgbotapi.NewMessage(update.Message.Chat.ID, "Вы уже зарегистрированы."))
		}
	}
}

// Метод, проверяющий статус регистрации пользователя
func (telegramBot *TelegramBot) isUserRegistered(userID int64) bool {
	var existingUserID int
	err := db.QueryRow("SELECT telegram_id FROM users WHERE telegram_id = $1", userID).Scan(&existingUserID)
	return err != sql.ErrNoRows
}

// Метод отправляет клавиатуру "Регистрация" только если пользователь не зарегистрирован
func (telegramBot *TelegramBot) sendMainMenu(userID int64) {
	msg := tgbotapi.NewMessage(userID, "Добро пожаловать! Выберите действие:")
	keyboard := tgbotapi.NewReplyKeyboard(
		tgbotapi.NewKeyboardButtonRow(
			tgbotapi.NewKeyboardButton("Вход"),
			tgbotapi.NewKeyboardButton("Регистрация"),
		),
	)
	msg.ReplyMarkup = keyboard
	_, err := telegramBot.API.Send(msg)
	if err != nil {
		log.Println("Error sending main menu:", err)
	}
}

func (telegramBot *TelegramBot) registerUser(userID int64) {
	// Скрываем клавиатуру перед началом регистрации
	hideKeyboard := tgbotapi.NewRemoveKeyboard(false)
	msg := tgbotapi.NewMessage(userID, "Введите вашу фамилию:")
	msg.ReplyMarkup = hideKeyboard
	telegramBot.API.Send(msg)

	// Проверяем, зарегистрирован ли пользователь
	if telegramBot.isUserRegistered(userID) {
		// Если пользователь уже зарегистрирован, отправляем сообщение об этом.
		msg := tgbotapi.NewMessage(userID, "Вы уже зарегистрированы.")
		telegramBot.API.Send(msg)
		return
	}

	// Получаем фамилию от пользователя
	update := <-telegramBot.Updates
	if update.Message == nil {
		log.Println("Ошибка получения фамилии от пользователя")
		return
	}
	lastName := update.Message.Text

	// Получаем имя пользователя
	msg = tgbotapi.NewMessage(userID, "Введите ваше имя:")
	telegramBot.API.Send(msg)

	update = <-telegramBot.Updates
	if update.Message == nil {
		log.Println("Ошибка получения имени от пользователя")
		return
	}
	firstName := update.Message.Text

	// Показываем клавиатуру с кнопками выбора курса
	msg = tgbotapi.NewMessage(userID, "Выберите ваш курс:")
	courseKeyboard := tgbotapi.NewReplyKeyboard([]tgbotapi.KeyboardButton{
		{Text: "1 курс"},
		{Text: "2 курс"},
		{Text: "3 курс"},
		{Text: "4 курс"},
		{Text: "5 курс"},
		{Text: "6 курс"},
	})
	msg.ReplyMarkup = courseKeyboard
	telegramBot.API.Send(msg)

	// Получаем курс пользователя
	update = <-telegramBot.Updates
	if update.Message == nil {
		log.Println("Ошибка получения курса от пользователя")
		return
	}
	course := update.Message.Text

	// Скрываем клавиатуру после выбора курса
	hideKeyboard = tgbotapi.NewRemoveKeyboard(true)
	msg.ReplyMarkup = hideKeyboard

	// Показываем клавиатуру с кнопками "иб" и "кб"
	msg = tgbotapi.NewMessage(userID, "Введите ваше направление:")
	groupKeyboard := tgbotapi.NewReplyKeyboard([]tgbotapi.KeyboardButton{
		{Text: "ИБ"},
		{Text: "КБ"},
	})
	msg.ReplyMarkup = groupKeyboard
	telegramBot.API.Send(msg)

	// Получаем группу пользователя
	update = <-telegramBot.Updates
	if update.Message == nil {
		log.Println("Ошибка получения группы от пользователя")
		return
	}
	groupName := update.Message.Text

	// Скрываем клавиатуру после выбора группы
	hideKeyboard = tgbotapi.NewRemoveKeyboard(true)
	msg = tgbotapi.NewMessage(userID, "Введите ваш пароль:")
	msg.ReplyMarkup = hideKeyboard
	telegramBot.API.Send(msg)

	// Получаем пароль пользователя
	update = <-telegramBot.Updates
	if update.Message == nil {
		log.Println("Ошибка получения пароля от пользователя")
		return
	}
	password := update.Message.Text

	// Регистрируем пользователя с введенными данными.
	_, err := db.Exec("INSERT INTO users (telegram_id, last_name, first_name, course, group_name, password) VALUES ($1, $2, $3, $4, $5, $6)",
		userID, lastName, firstName, course, groupName, password)
	if err != nil {
		log.Println("Ошибка регистрации пользователя:", err)
		return
	}

	// Отправляем сообщение о успешной регистрации
	msg = tgbotapi.NewMessage(userID, "Вы успешно зарегистрированы.")
	telegramBot.API.Send(msg)
}

// Функция для обработки команды "Вход".
func (telegramBot *TelegramBot) handleLogin(chatID int64) {
	// Проверяем, есть ли пользователь уже в базе данных.
	var existingUserID int
	err := db.QueryRow("SELECT telegram_id FROM users WHERE telegram_id = $1", chatID).Scan(&existingUserID)
	switch {
	case err == sql.ErrNoRows:
		// Если пользователя нет в базе данных, сообщаем ему об этом.
		msg := tgbotapi.NewMessage(chatID, "Вы не зарегистрированы. Используйте /start для регистрации.")
		telegramBot.API.Send(msg)
	case err != nil:
		log.Println("Error checking user:", err)
		msg := tgbotapi.NewMessage(chatID, "Error during login. Please try again.")
		telegramBot.API.Send(msg)
	default:
		// Если пользователь зарегистрирован, запрашиваем пароль.
		msg := tgbotapi.NewMessage(chatID, "Введите ваш пароль:")
		telegramBot.API.Send(msg)

		// Устанавливаем состояние ожидания ввода пароля.
		telegramBot.UserStates[chatID] = "waiting_login_password"
	}
}

// Функция для обработки ответа на запрос пароля при входе.
func (telegramBot *TelegramBot) handleLoginPasswordResponse(chatID int64, text string) {
	// Здесь вы можете проверить пароль в базе данных.
	// В этом примере просто сообщаем о успешном входе.

	// Ваша логика проверки пароля может быть более сложной.

	// В данном примере считаем, что пароль совпал, и отправляем сообщение об успешном входе.
	msg := tgbotapi.NewMessage(chatID, "Вы успешно вошли.")
	telegramBot.API.Send(msg)

	// Сбрасываем состояние пользователя.
	delete(telegramBot.UserStates, chatID)
}

// ... (ваш импорт)

// Структура для хранения информации о пользователях.
type User struct {
	ID         int
	TelegramID int
	LastName   string
	FirstName  string
	Course     string
	GroupName  string
	Password   string
}

// Структура для хранения информации об администраторе.
type Admin struct {
	// Добавьте необходимые поля для администратора
}

// Структура для управления ботом.
type TelegramBot struct {
	// ... (ваша текущая структура)

	// Добавьте поле для хранения администраторов.
	Admins map[int64]*Admin
}

// Инициализация бота и базы данных при запуске программы.
func init() {
	// ... (ваш код)
	// Создаем таблицу администраторов, если она еще не существует.
	createAdminTableQuery := `
CREATE TABLE IF NOT EXISTS admins (
	id SERIAL PRIMARY KEY,
	telegram_id INT UNIQUE,
	password VARCHAR(255)
);
`
	_, err = db.Exec(createAdminTableQuery)
	if err != nil {
		log.Fatal(err)
	
// Структура для хранения информации о пользователях.
type User struct {
	ID         int
	TelegramID int
	LastName   string
	FirstName  string
	Course     string
	GroupName  string
	Password   string
}

// Структура для хранения информации об администраторе.
type Admin struct {
	ID         int
	TelegramID int
	Password   string
}

// Структура для управления ботом.
type TelegramBot struct {
	API                   *tgbotapi.BotAPI
	Updates               tgbotapi.UpdatesChannel
	ActiveContactRequests []int64
	UserStates            map[int64]string
	Admins                map[int64]*Admin
}

	// Создаем таблицу администраторов, если она еще не существует.
	createAdminTableQuery := `
CREATE TABLE IF NOT EXISTS admins (
	id SERIAL PRIMARY KEY,
	telegram_id INT UNIQUE,
	password VARCHAR(255)
);
`
	_, err = db.Exec(createAdminTableQuery)
	if err != nil {
		log.Fatal(err)
	}
}

// Обработчик команды "/admin".
func (telegramBot *TelegramBot) handleAdminCommand(userID int64) {
	// Проверяем, является ли пользователь администратором.
	if _, isAdmin := telegramBot.Admins[userID]; isAdmin {
		// Пользователь уже администратор.
		// Реализуйте здесь логику администратора, например, отображение меню администратора.
	} else {
		// Пользователь не является администратором.
		// Запрашиваем пароль для подтверждения статуса администратора.
		msg := tgbotapi.NewMessage(userID, "Введите пароль администратора:")
		telegramBot.API.Send(msg)

		// Ждем ответа пользователя и проверяем пароль в базе данных.
		update := <-telegramBot.Updates
		if update.Message == nil {
			log.Println("Ошибка получения ответа от пользователя")
			return
		}
		password := update.Message.Text

		// Проверяем пароль администратора в базе данных.
		adminID, err := authenticateAdmin(userID, password)
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Неверный пароль администратора.")
			telegramBot.API.Send(msg)
			return
		}

		// Авторизация прошла успешно. Добавляем пользователя в список администраторов.
		telegramBot.Admins[userID] = &Admin{ID: adminID, TelegramID: userID, Password: password}

		// Реализуйте здесь логику администратора после успешной авторизации.
	}
}

// Функция для аутентификации администратора в базе данных.
func authenticateAdmin(telegramID int64, password string) (int, error) {
	var adminID int
	err := db.QueryRow("SELECT id FROM admins WHERE telegram_id = $1 AND password = $2", telegramID, password).Scan(&adminID)
	if err != nil {
		return 0, err
	}
	return adminID, nil
}

// ... (ваш дополнительный код)

// Структура для хранения информации о лабораторных работах.
type Lab struct {
	ID     int
	Name   string
	Number int
	// Добавьте другие необходимые поля
}

// Структура для хранения результатов лабораторных работ.
type CheckLab struct {
	ID      int
	UserID  int
	LabID   int
	Mark    int
	Passed  bool
	// Добавьте другие необходимые поля
}

// ... (ваш код)

// Инициализация базы данных при запуске программы.
func init() {
	// ... (ваш код)
	// Создаем таблицу лабораторных работ, если она еще не существует.
	createLabTableQuery := `
CREATE TABLE IF NOT EXISTS labs (
	id SERIAL PRIMARY KEY,
	name VARCHAR(255),
	number INT
);
`
	_, err = db.Exec(createLabTableQuery)
	if err != nil {
		log.Fatal(err)
	}

	// Создаем таблицу результатов лабораторных работ, если она еще не существует.
	createCheckLabTableQuery := `
CREATE TABLE IF NOT EXISTS check_labs (
	id SERIAL PRIMARY KEY,
	user_id INT,
	lab_id INT,
	mark INT,
	passed BOOLEAN
);
`
	_, err = db.Exec(createCheckLabTableQuery)
	if err != nil {
		log.Fatal(err)
	}
}

// Функция для добавления лабораторной работы в базу данных.
func addLab(name string, number int) error {
	_, err := db.Exec("INSERT INTO labs (name, number) VALUES ($1, $2)", name, number)
	return err
}

// Функция для добавления результата лабораторной работы в базу данных.
func addCheckLab(userID, labID, mark int, passed bool) error {
	_, err := db.Exec("INSERT INTO check_labs (user_id, lab_id, mark, passed) VALUES ($1, $2, $3, $4)", userID, labID, mark, passed)
	return err
}

// ... (ваш дополнительный код)

// Функция для обработки команды пользователя по добавлению лабораторной работы.
func (telegramBot *TelegramBot) handleAddLabCommand(userID int64) {
	// Проверяем, является ли пользователь администратором.
	if admin, isAdmin := telegramBot.Admins[userID]; isAdmin {
		// Пользователь администратор.
		msg := tgbotapi.NewMessage(userID, "Введите название и номер лабораторной работы через пробел:")
		telegramBot.API.Send(msg)

		// Ждем ответа пользователя.
		update := <-telegramBot.Updates
		if update.Message == nil {
			log.Println("Ошибка получения ответа от пользователя")
			return
		}
		input := update.Message.Text

		// Разбиваем ввод пользователя на название и номер лабораторной работы.
		parts := strings.Fields(input)
		if len(parts) != 2 {
			msg := tgbotapi.NewMessage(userID, "Неверный формат ввода. Попробуйте снова.")
			telegramBot.API.Send(msg)
			return
		}

		name := parts[0]
		number, err := strconv.Atoi(parts[1])
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Неверный формат номера лабораторной работы. Попробуйте снова.")
			telegramBot.API.Send(msg)
			return
		}

		// Добавляем лабораторную работу в базу данных.
		err = addLab(name, number)
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Ошибка добавления лабораторной работы в базу данных.")
			telegramBot.API.Send(msg)
			return
		}

		msg := tgbotapi.NewMessage(userID, "Лабораторная работа успешно добавлена.")
		telegramBot.API.Send(msg)
	} else {
		// Пользователь не является администратором.
		msg := tgbotapi.NewMessage(userID, "Вы не являетесь администратором.")
		telegramBot.API.Send(msg)
	}
}

// Функция для обработки команды пользователя по выставлению оценки за лабораторную работу.
func (telegramBot *TelegramBot) handleMarkLabCommand(userID int64) {
	// Проверяем, является ли пользователь администратором.
	if admin, isAdmin := telegramBot.Admins[userID]; isAdmin {
		// Пользователь администратор.
		msg := tgbotapi.NewMessage(userID, "Введите ID пользователя, ID лабораторной работы, оценку и результат (зачет/незачет) через пробел:")
		telegramBot.API.Send(msg)

		// Ждем ответа пользователя.
		update := <-telegramBot.Updates
		if update.Message == nil {
			log.Println("Ошибка получения ответа от пользователя")
			return
		}
		input := update.Message.Text

		// Разбиваем ввод пользователя.
		parts := strings.Fields(input)
		if len(parts) != 4 {
			msg := tgbotapi.NewMessage(userID, "Неверный формат ввода. Попробуйте снова.")
			telegramBot.API.Send(msg)
			return
		}

		userID, err := strconv.Atoi(parts[0])
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Неверный формат ID пользователя. Попробуйте снова.")
			telegramBot.API.Send(msg)
			return
		}

		labID, err := strconv.Atoi(parts[1])
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Неверный формат ID лабораторной работы. Попробуйте снова.")
			telegramBot.API.Send(msg)
			return
		}

		mark, err := strconv.Atoi(parts[2])
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Неверный формат оценки. Попробуйте снова.")
			telegramBot.API.Send(msg)
			return
		}

		passed := parts[3] == "зачет"

		// Добавляем результат лабораторной работы в базу данных.
		err = addCheckLab(userID, labID, mark, passed)
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Ошибка добавления результата лабораторной работы в базу данных.")
			telegramBot.API.Send(msg)
			return
		}

		msg := tgbotapi.NewMessage(userID, "Оценка успешно выставлена.")
		telegramBot.API.Send(msg)
	} else {
		// Пользователь не является администратором.
		msg := tgbotapi.NewMessage(userID, "Вы не являетесь администратором.")
		telegramBot.API.Send(msg)
	}
}

// Функция для обработки команды администратора по выставлению оценки за лабораторную работу.
func (telegramBot *TelegramBot) handleMarkLabCommand(userID int64) {
	// Проверяем, является ли пользователь администратором.
	if admin, isAdmin := telegramBot.Admins[userID]; isAdmin {
		// Пользователь администратор.
		msg := tgbotapi.NewMessage(userID, "Введите ID пользователя, ID лабораторной работы, оценку и результат (зачет/незачет) через пробел:")
		telegramBot.API.Send(msg)

		// Ждем ответа пользователя.
		update := <-telegramBot.Updates
		if update.Message == nil {
			log.Println("Ошибка получения ответа от пользователя")
			return
		}
		input := update.Message.Text

		// Разбиваем ввод пользователя.
		parts := strings.Fields(input)
		if len(parts) != 4 {
			msg := tgbotapi.NewMessage(userID, "Неверный формат ввода. Попробуйте снова.")
			telegramBot.API.Send(msg)
			return
		}

		userID, err := strconv.Atoi(parts[0])
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Неверный формат ID пользователя. Попробуйте снова.")
			telegramBot.API.Send(msg)
			return
		}

		labID, err := strconv.Atoi(parts[1])
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Неверный формат ID лабораторной работы. Попробуйте снова.")
			telegramBot.API.Send(msg)
			return
		}

		mark, err := strconv.Atoi(parts[2])
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Неверный формат оценки. Попробуйте снова.")
			telegramBot.API.Send(msg)
			return
		}

		passed := parts[3] == "зачет"

		// Добавляем результат лабораторной работы в базу данных.
		err = addCheckLab(userID, labID, mark, passed)
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Ошибка добавления результата лабораторной работы в базу данных.")
			telegramBot.API.Send(msg)
			return
		}

		msg := tgbotapi.NewMessage(userID, "Оценка успешно выставлена.")
		telegramBot.API.Send(msg)
	} else {
		// Пользователь не является администратором.
		msg := tgbotapi.NewMessage(userID, "Вы не являетесь администратором.")
		telegramBot.API.Send(msg)
	}
}

func (telegramBot *TelegramBot) handleViewResultsCommand(userID int64) {
	// Получаем ID пользователя.
	userID, err := getUserIDByTelegramID(userID)
	if err != nil {
		msg := tgbotapi.NewMessage(userID, "Ошибка получения ID пользователя.")
		telegramBot.API.Send(msg)
		return
	}

	// Запрос к базе данных для получения результатов пользователя.
	rows, err := db.Query("SELECT labs.name, check_labs.mark, check_labs.passed FROM check_labs JOIN labs ON check_labs.lab_id = labs.id WHERE check_labs.user_id = $1", userID)
	if err != nil {
		msg := tgbotapi.NewMessage(userID, "Ошибка получения результатов из базы данных.")
		telegramBot.API.Send(msg)
		return
	}
	defer rows.Close()

	// Формируем сообщение с результатами.
	var resultsMessage string
	for rows.Next() {
		var labName string
		var mark int
		var passed bool
		err := rows.Scan(&labName, &mark, &passed)
		if err != nil {
			log.Println("Ошибка сканирования результатов:", err)
			continue
		}

		result := fmt.Sprintf("%s: %d (%s)\n", labName, mark, getResultStatus(passed))
		resultsMessage += result
	}

	if resultsMessage == "" {
		resultsMessage = "У вас пока нет результатов лабораторных работ."
	}

	msg := tgbotapi.NewMessage(userID, resultsMessage)
	telegramBot.API.Send(msg)
}

// Функция для получения статуса результата (зачет/незачет).
func getResultStatus(passed bool) string {
	if passed {
		return "зачет"
	}
	return "незачет"
}

// Функция для получения ID пользователя по его Telegram ID.
func getUserIDByTelegramID(telegramID int64) (int, error) {
	var userID int
	err := db.QueryRow("SELECT id FROM users WHERE telegram_id = $1", telegramID).Scan(&userID)
	return userID, err
}

// ... (ваш код продолжается)


// Функция для просмотра результатов лабораторных работ пользователя.
func (telegramBot *TelegramBot) handleAdminViewResultsCommand(userID int64) {
	// Проверяем, является ли пользователь администратором.
	if admin, isAdmin := telegramBot.Admins[userID]; isAdmin {
		// Пользователь администратор.
		msg := tgbotapi.NewMessage(userID, "Введите ID пользователя для просмотра его результатов:")
		telegramBot.API.Send(msg)

		// Ждем ответа пользователя.
		update := <-telegramBot.Updates
		if update.Message == nil {
			log.Println("Ошибка получения ответа от пользователя")
			return
		}
		input := update.Message.Text

		// Получаем ID пользователя.
		userID, err := strconv.Atoi(input)
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Неверный формат ID пользователя. Попробуйте снова.")
			telegramBot.API.Send(msg)
			return
		}

		// Запрос к базе данных для получения результатов пользователя.
		rows, err := db.Query("SELECT labs.name, check_labs.mark, check_labs.passed FROM check_labs JOIN labs ON check_labs.lab_id = labs.id WHERE check_labs.user_id = $1", userID)
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Ошибка получения результатов из базы данных.")
			telegramBot.API.Send(msg)
			return
		}
		defer rows.Close()

		// Формируем сообщение с результатами.
		var resultsMessage string
		for rows.Next() {
			var labName string
			var mark int
			var passed bool
			err := rows.Scan(&labName, &mark, &passed)
			if err != nil {
				log.Println("Ошибка сканирования результатов:", err)
				continue
			}

			result := fmt.Sprintf("%s: %d (%s)\n", labName, mark, getResultStatus(passed))
			resultsMessage += result
		}

		if resultsMessage == "" {
			resultsMessage = "У пользователя пока нет результатов лабораторных работ."
		}

		msg := tgbotapi.NewMessage(userID, resultsMessage)
		telegramBot.API.Send(msg)
	} else {
		// Пользователь не является администратором.
		msg := tgbotapi.NewMessage(userID, "Вы не являетесь администратором.")
		telegramBot.API.Send(msg)
	}
}

// ... (ваш код продолжается)

// Функция для просмотра всех пользователей и их групп.
func (telegramBot *TelegramBot) handleAdminViewUsersCommand(userID int64) {
	// Проверяем, является ли пользователь администратором.
	if admin, isAdmin := telegramBot.Admins[userID]; isAdmin {
		// Пользователь администратор.
		rows, err := db.Query("SELECT id, telegram_id, last_name, first_name, course, group_name FROM users")
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Ошибка получения списка пользователей из базы данных.")
			telegramBot.API.Send(msg)
			return
		}
		defer rows.Close()

		// Формируем сообщение с пользователями и их группами.
		var usersMessage string
		for rows.Next() {
			var id int
			var telegramID int64
			var lastName, firstName, course, groupName string
			err := rows.Scan(&id, &telegramID, &lastName, &firstName, &course, &groupName)
			if err != nil {
				log.Println("Ошибка сканирования пользователей:", err)
				continue
			}

			user := fmt.Sprintf("ID: %d\nTelegram ID: %d\nФамилия: %s\nИмя: %s\nКурс: %s\nГруппа: %s\n\n", id, telegramID, lastName, firstName, course, groupName)
			usersMessage += user
		}

		if usersMessage == "" {
			usersMessage = "Пользователей пока нет."
		}

		msg := tgbotapi.NewMessage(userID, usersMessage)
		telegramBot.API.Send(msg)
	} else {
		// Пользователь не является администратором.
		msg := tgbotapi.NewMessage(userID, "Вы не являетесь администратором.")
		telegramBot.API.Send(msg)
	}
}

// Функция для обновления статуса пользователя (находится в учебном заведении или нет).
func (telegramBot *TelegramBot) handleUpdateUserStatusCommand(userID int64) {
	// Проверяем, является ли пользователь администратором.
	if admin, isAdmin := telegramBot.Admins[userID]; isAdmin {
		// Пользователь администратор.
		msg := tgbotapi.NewMessage(userID, "Введите ID пользователя и новый статус (true/false) через пробел:")
		telegramBot.API.Send(msg)

		// Ждем ответа пользователя.
		update := <-telegramBot.Updates
		if update.Message == nil {
			log.Println("Ошибка получения ответа от пользователя")
			return
		}
		input := update.Message.Text

		// Разбиваем ввод пользователя.
		parts := strings.Fields(input)
		if len(parts) != 2 {
			msg := tgbotapi.NewMessage(userID, "Неверный формат ввода. Попробуйте снова.")
			telegramBot.API.Send(msg)
			return
		}

		userID, err := strconv.Atoi(parts[0])
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Неверный формат ID пользователя. Попробуйте снова.")
			telegramBot.API.Send(msg)
			return
		}

		status, err := strconv.ParseBool(parts[1])
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Неверный формат статуса. Попробуйте снова.")
			telegramBot.API.Send(msg)
			return
		}

		// Обновляем статус пользователя в базе данных.
		err = updateUserStatus(userID, status)
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Ошибка обновления статуса пользователя в базе данных.")
			telegramBot.API.Send(msg)
			return
		}

		msg := tgbotapi.NewMessage(userID, "Статус пользователя успешно обновлен.")
		telegramBot.API.Send(msg)
	} else {
		// Пользователь не является администратором.
		msg := tgbotapi.NewMessage(userID, "Вы не являетесь администратором.")
		telegramBot.API.Send(msg)
	}
}

// Функция для обновления статуса пользователя в базе данных.
func updateUserStatus(userID int, status bool) error {
	_, err := db.Exec("UPDATE users SET in_educational_institution = $1 WHERE id = $2", status, userID)
	return err
}

// ... (ваш код продолжается)

// Функция для просмотра списка лабораторных работ.
func (telegramBot *TelegramBot) handleAdminViewLabsCommand(userID int64) {
	// Проверяем, является ли пользователь администратором.
	if admin, isAdmin := telegramBot.Admins[userID]; isAdmin {
		// Пользователь администратор.
		rows, err := db.Query("SELECT id, name FROM labs")
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Ошибка получения списка лабораторных работ из базы данных.")
			telegramBot.API.Send(msg)
			return
		}
		defer rows.Close()

		// Формируем сообщение со списком лабораторных работ.
		var labsMessage string
		for rows.Next() {
			var id int
			var name string
			err := rows.Scan(&id, &name)
			if err != nil {
				log.Println("Ошибка сканирования лабораторных работ:", err)
				continue
			}

			lab := fmt.Sprintf("ID: %d\nНазвание: %s\n\n", id, name)
			labsMessage += lab
		}

		if labsMessage == "" {
			labsMessage = "Лабораторных работ пока нет."
		}

		msg := tgbotapi.NewMessage(userID, labsMessage)
		telegramBot.API.Send(msg)
	} else {
		// Пользователь не является администратором.
		msg := tgbotapi.NewMessage(userID, "Вы не являетесь администратором.")
		telegramBot.API.Send(msg)
	}
}

// Функция для просмотра результатов всех студентов по лабораторной работе.
func (telegramBot *TelegramBot) handleAdminViewLabResultsCommand(userID int64) {
	// Проверяем, является ли пользователь администратором.
	if admin, isAdmin := telegramBot.Admins[userID]; isAdmin {
		// Пользователь администратор.
		msg := tgbotapi.NewMessage(userID, "Введите ID лабораторной работы:")
		telegramBot.API.Send(msg)

		// Ждем ответа пользователя.
		update := <-telegramBot.Updates
		if update.Message == nil {
			log.Println("Ошибка получения ответа от пользователя")
			return
		}
		input := update.Message.Text

		// Получаем ID лабораторной работы.
		labID, err := strconv.Atoi(input)
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Неверный формат ID лабораторной работы. Попробуйте снова.")
			telegramBot.API.Send(msg)
			return
		}

		// Запрос к базе данных для получения результатов всех студентов по лабораторной работе.
		rows, err := db.Query("SELECT users.last_name, users.first_name, check_labs.mark, check_labs.passed FROM check_labs JOIN users ON check_labs.user_id = users.id WHERE check_labs.lab_id = $1", labID)
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Ошибка получения результатов из базы данных.")
			telegramBot.API.Send(msg)
			return
		}
		defer rows.Close()

		// Формируем сообщение с результатами.
		var resultsMessage string
		for rows.Next() {
			var lastName, firstName string
			var mark int
			var passed bool
			err := rows.Scan(&lastName, &firstName, &mark, &passed)
			if err != nil {
				log.Println("Ошибка сканирования результатов:", err)
				continue
			}

			result := fmt.Sprintf("%s %s: %d (%s)\n", lastName, firstName, mark, getResultStatus(passed))
			resultsMessage += result
		}

		if resultsMessage == "" {
			resultsMessage = "Результатов пока нет."
		}

		msg := tgbotapi.NewMessage(userID, resultsMessage)
		telegramBot.API.Send(msg)
	} else {
		// Пользователь не является администратором.
		msg := tgbotapi.NewMessage(userID, "Вы не являетесь администратором.")
		telegramBot.API.Send(msg)
	}
}

// Функция для выставления оценки студенту за лабораторную работу.
func (telegramBot *TelegramBot) handleAdminSetMarkCommand(userID int64) {
	// Проверяем, является ли пользователь администратором.
	if admin, isAdmin := telegramBot.Admins[userID]; isAdmin {
		// Пользователь администратор.
		msg := tgbotapi.NewMessage(userID, "Введите ID пользователя, ID лабораторной работы и оценку через пробел:")
		telegramBot.API.Send(msg)

		// Ждем ответа пользователя.
		update := <-telegramBot.Updates
		if update.Message == nil {
			log.Println("Ошибка получения ответа от пользователя")
			return
		}
		input := update.Message.Text

		// Разбиваем ввод пользователя.
		parts := strings.Fields(input)
		if len(parts) != 3 {
			msg := tgbotapi.NewMessage(userID, "Неверный формат ввода. Попробуйте снова.")
			telegramBot.API.Send(msg)
			return
		}

		userID, err := strconv.Atoi(parts[0])
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Неверный формат ID пользователя. Попробуйте снова.")
			telegramBot.API.Send(msg)
			return
		}

		labID, err := strconv.Atoi(parts[1])
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Неверный формат ID лабораторной работы. Попробуйте снова.")
			telegramBot.API.Send(msg)
			return
		}

		mark, err := strconv.Atoi(parts[2])
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Неверный формат оценки. Попробуйте снова.")
			telegramBot.API.Send(msg)
			return
		}

		// Обновляем оценку студента в базе данных.
		err = setMark(userID, labID, mark)
		if err != nil {
			msg := tgbotapi.NewMessage(userID, "Ошибка выставления оценки в базе данных.")
			telegramBot.API.Send(msg)
			return
		}

		msg := tgbotapi.NewMessage(userID, "Оценка успешно выставлена.")
		telegramBot.API.Send(msg)
	} else {
		// Пользователь не является администратором.
		msg := tgbotapi.NewMessage(userID, "Вы не являетесь администратором.")
		telegramBot.API.Send(msg)
	}
}

// Функция для выставления оценки студенту в базе данных.
func setMark(userID, labID, mark int) error {
	_, err := db.Exec("UPDATE check_labs SET mark = $1 WHERE user_id = $2 AND lab_id = $3", mark, userID, labID)
	return err
}

// Функция для определения статуса результата (сдал/не сдал).
func getResultStatus(passed bool) string {
	if passed {
		return "сдал"
	}
	return "не сдал"
}

createLabsTableQuery := `
CREATE TABLE IF NOT EXISTS labs (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);
`

_, err = db.Exec(createLabsTableQuery)
if err != nil {
    log.Fatal(err)
}
"check_labs":

createCheckLabsTableQuery := `
CREATE TABLE IF NOT EXISTS check_labs (
    id SERIAL PRIMARY KEY,
    user_id INT,
    lab_id INT,
    mark INT,
    passed BOOLEAN,
    FOREIGN KEY (user_id) REFERENCES users(id),
    FOREIGN KEY (lab_id) REFERENCES labs(id)
);
`

_, err = db.Exec(createCheckLabsTableQuery)
if err != nil {
    log.Fatal(err)
}

// Ваша структура для отслеживания состояний пользователя и обновленная структура для администратора.
type TelegramBot struct {
    // ... (ваш код)

    // Добавим слайс для хранения информации о лабораторных работах.
    Labs []Lab
}

type Lab struct {
    ID   int
    Name string
}

// Функция для обработки команды "Просмотр лабораторных работ".
func (telegramBot *TelegramBot) handleViewLabsCommand(userID int64) {
    // Проверяем, зарегистрирован ли пользователь.
    if telegramBot.isUserRegistered(userID) {
        // Получаем список лабораторных работ из базы данных.
        labs, err := getLabs()
        if err != nil {
            msg := tgbotapi.NewMessage(userID, "Ошибка получения списка лабораторных работ из базы данных.")
            telegramBot.API.Send(msg)
            return
        }

        // Формируем сообщение со списком лабораторных работ.
        var labsMessage string
        for _, lab := range labs {
            labsMessage += fmt.Sprintf("ID: %d\nНазвание: %s\n\n", lab.ID, lab.Name)
        }

        if labsMessage == "" {
            labsMessage = "Лабораторных работ пока нет."
        }

        msg := tgbotapi.NewMessage(userID, labsMessage)
        telegramBot.API.Send(msg)
    } else {
        // Если пользователь не зарегистрирован, отправляем сообщение об этом.
        msg := tgbotapi.NewMessage(userID, "Вы не зарегистрированы. Используйте /start для регистрации.")
        telegramBot.API.Send(msg)
    }
}

// Функция для получения списка лабораторных работ из базы данных.
func getLabs() ([]Lab, error) {
    rows, err := db.Query("SELECT id, name FROM labs")
    if err != nil {
        return nil, err
    }
    defer rows.Close()

    var labs []Lab
    for rows.Next() {
        var lab Lab
        err := rows.Scan(&lab.ID, &lab.Name)
        if err != nil {
            log.Println("Ошибка сканирования лабораторных работ:", err)
            continue
        }
        labs = append(labs, lab)
    }

    return labs, nil
}