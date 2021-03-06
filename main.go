package main

import (
	"net/http"
	"os"
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/caarlos0/alelobot/internal/alelo"
	"github.com/caarlos0/alelobot/internal/datastore"
	"github.com/caarlos0/alelogo"
	"github.com/go-telegram-bot-api/telegram-bot-api"
)

func main() {
	ds := datastore.NewRedis(os.Getenv("REDIS_URL"))
	defer ds.Close()

	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_TOKEN"))
	if err != nil {
		log.Panic(err)
	}
	log.Printf("Authorized on account %s", bot.Self.UserName)

	// without a port binded, heroku complains and eventually kills the process.
	go serve()

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates, err := bot.GetUpdatesChan(u)
	if err != nil {
		log.Panic(err)
	}

	for update := range updates {
		update := update
		if update.Message == nil {
			continue
		}
		log.WithFields(log.Fields{
			"ChatID": update.Message.Chat.ID,
			"From":   update.Message.From.UserName,
		}).Info("New Message")
		if update.Message.Command() == "login" {
			login(ds, bot, update)
			continue
		}
		if update.Message.Command() == "balance" {
			go balance(ds, bot, update)
			continue
		}
		log.WithFields(log.Fields{
			"ChatID": update.Message.Chat.ID,
			"From":   update.Message.From.UserName,
		}).Info("Unknown command", update.Message.Text)
		bot.Send(tgbotapi.NewMessage(
			update.Message.Chat.ID,
			"Os únicos comandos suportados são /login e /balance",
		))
	}
}

func balance(ds datastore.Datastore, bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	cpf, pwd, err := ds.Retrieve(update.Message.From.ID)
	if cpf == "" || pwd == "" || err != nil {
		log.WithFields(log.Fields{
			"ChatID": update.Message.Chat.ID,
			"From":   update.Message.From.UserName,
		}).Info("Not logged in, telling user to do that")
		bot.Send(tgbotapi.NewMessage(
			update.Message.Chat.ID,
			"Por favor, faça /login...",
		))
		return
	}
	details, err := alelo.AllDetails(cpf, pwd)
	if err != nil {
		bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, err.Error()))
		return
	}
	for _, detail := range details {
		log.WithFields(log.Fields{
			"ChatID":  update.Message.Chat.ID,
			"From":    update.Message.From.UserName,
			"cpf":     cpf,
			"card_id": detail.Number,
		}).Info("Got user card's details", details)
		bot.Send(tgbotapi.NewMessage(
			update.Message.Chat.ID,
			"Saldo do cartão "+detail.Number+" é "+detail.Balance,
		))
	}
}

func login(ds datastore.Datastore, bot *tgbotapi.BotAPI, update tgbotapi.Update) {
	parts := strings.Split(
		strings.TrimSpace(update.Message.CommandArguments()), " ",
	)
	if len(parts) != 2 {
		bot.Send(tgbotapi.NewMessage(
			update.Message.Chat.ID,
			"Para fazer login, diga\n\n/login CPF Senha",
		))
		return
	}
	cpf, pwd := parts[0], parts[1]
	if _, err := alelogo.New(cpf, pwd); err != nil {
		log.WithFields(log.Fields{
			"ChatID": update.Message.Chat.ID,
			"From":   update.Message.From.UserName,
			"cpf":    cpf,
		}).Error(err.Error())
		bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, err.Error()))
		return
	}
	if err := ds.Save(update.Message.From.ID, cpf, pwd); err != nil {
		log.WithFields(log.Fields{
			"ChatID": update.Message.Chat.ID,
			"From":   update.Message.From.UserName,
			"cpf":    cpf,
		}).Error(err.Error())
		bot.Send(tgbotapi.NewMessage(update.Message.Chat.ID, err.Error()))
		return
	}
	log.WithFields(log.Fields{
		"ChatID": update.Message.Chat.ID,
		"From":   update.Message.From.UserName,
		"cpf":    cpf,
	}).Info("Login success")
	bot.Send(tgbotapi.NewMessage(
		update.Message.Chat.ID, "Sucesso, agora é só dizer /balance!",
	))
}

func serve() {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		cpf := os.Getenv("TEST_CPF")
		pwd := os.Getenv("TEST_PWD")
		_, err := alelo.AllDetails(cpf, pwd)
		if err != nil {
			http.Error(
				w,
				"Can't connect to Alelo API",
				http.StatusServiceUnavailable,
			)
			return
		}
		w.Write([]byte("OK"))
	})
	http.ListenAndServe(":"+os.Getenv("PORT"), nil)
}
