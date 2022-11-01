package handlers

import (
	dto "dumbmerch/dto/result"
	transactiondto "dumbmerch/dto/transaction"
	"dumbmerch/models"
	"dumbmerch/repositories"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"

	"github.com/golang-jwt/jwt/v4"
	"github.com/midtrans/midtrans-go"         // import midtrans-go package ...
	"github.com/midtrans/midtrans-go/coreapi" // import midtrans-go/coreapi package ...
	"github.com/midtrans/midtrans-go/snap"    // import midtrans-go/snap package ...
	"gopkg.in/gomail.v2"                      // import gomail package ...
)

// Declare Coreapi Client ...
var c = coreapi.Client{
	ServerKey: os.Getenv("SERVER_KEY"),
	ClientKey:  os.Getenv("CLIENT_KEY"),
}

type handlerTransaction struct {
	TransactionRepository repositories.TransactionRepository
}

func HandlerTransaction(TransactionRepository repositories.TransactionRepository) *handlerTransaction {
	return &handlerTransaction{TransactionRepository}
}

func (h *handlerTransaction) FindTransactions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userInfo := r.Context().Value("userInfo").(jwt.MapClaims)
	userId := int(userInfo["id"].(float64))

	transactions, err := h.TransactionRepository.FindTransactions(userId)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
	}

	var responseTransaction []transactiondto.TransactionResponse
	for _, t := range transactions {
		responseTransaction = append(responseTransaction, convertResponseTransaction(t))
	}

	for i, t := range responseTransaction {
		imagePath := os.Getenv("PATH_FILE") + t.Product.Image
		responseTransaction[i].Product.Image = imagePath
	}

	w.WriteHeader(http.StatusOK)
	response := dto.SuccessResult{Code: http.StatusOK, Data: responseTransaction}
	json.NewEncoder(w).Encode(response)
}

func (h *handlerTransaction) CreateTransaction(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	userInfo := r.Context().Value("userInfo").(jwt.MapClaims)
	userId := int(userInfo["id"].(float64))

	var request transactiondto.TransactionRequest
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		response := dto.ErrorResult{Code: http.StatusBadRequest, Message: err.Error()}
		json.NewEncoder(w).Encode(response)
		return
	}

	// Create Unique Transaction Id ...
	var TransIdIsMatch = false
	var TransactionId int
	for !TransIdIsMatch {
		TransactionId = userId + request.SellerId + request.ProductId + rand.Intn(10000) - rand.Intn(100)
		transactionData, _ := h.TransactionRepository.GetTransaction(TransactionId)
		if transactionData.ID == 0 {
			TransIdIsMatch = true
		}
	}

	transaction := models.Transaction{
		ID: TransactionId,
		ProductID: request.ProductId,
		BuyerID: userId,
		SellerID: request.SellerId,
		Price: request.Price,
		Status: "pending",
	}

	newTransaction, err := h.TransactionRepository.CreateTransaction(transaction)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
		return
	}

	dataTransactions, err := h.TransactionRepository.GetTransaction(newTransaction.ID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(err.Error())
		return
	}
	
	// Request payment token from midtrans ...
	// 1. Initiate Snap client
	var s = snap.Client{}
	s.New(os.Getenv("SERVER_KEY"), midtrans.Sandbox)
	// Use to midtrans.Production if you want Production Environment (accept real transaction).
	
	// 2. Initiate Snap request param
	req := &snap.Request{
		TransactionDetails: midtrans.TransactionDetails{
		  OrderID:  strconv.Itoa(dataTransactions.ID),
		  GrossAmt: int64(dataTransactions.Price),
		}, 
		CreditCard: &snap.CreditCardDetails{
		  Secure: true,
		},
		CustomerDetail: &midtrans.CustomerDetails{
		  FName: dataTransactions.Buyer.Name,
		  Email: dataTransactions.Buyer.Email,
		},
	  }
	
	// 3. Execute request create Snap transaction to Midtrans Snap API
	snapResp, _ := s.CreateTransaction(req)

	w.WriteHeader(http.StatusOK)
	response := dto.SuccessResult{Code: http.StatusOK, Data: snapResp}
	json.NewEncoder(w).Encode(response)
}

// Notification method ...
func (h *handlerTransaction) Notification(w http.ResponseWriter, r *http.Request) {
	var notificationPayload map[string]interface{}

	err := json.NewDecoder(r.Body).Decode(&notificationPayload)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		response := dto.ErrorResult{Code: http.StatusBadRequest, Message: err.Error()}
		json.NewEncoder(w).Encode(response)
		return
	}

	transactionStatus := notificationPayload["transaction_status"].(string)
	fraudStatus := notificationPayload["fraud_status"].(string)
	orderId := notificationPayload["order_id"].(string)

	transaction, _ := h.TransactionRepository.GetOneTransaction(orderId)

	if transactionStatus == "capture" {
		if fraudStatus == "challenge" {
			// TODO set transaction status on your database to 'challenge'
			// e.g: 'Payment status challenged. Please take action on your Merchant Administration Portal
			h.TransactionRepository.UpdateTransaction("pending",  orderId)
		} else if fraudStatus == "accept" {
			// TODO set transaction status on your database to 'success'
			SendMail("success", transaction)
			h.TransactionRepository.UpdateTransaction("success",  orderId)
		}
	} else if transactionStatus == "settlement" {
		// TODO set transaction status on your databaase to 'success'
		SendMail("success", transaction)
		h.TransactionRepository.UpdateTransaction("success",  orderId)
	} else if transactionStatus == "deny" {
		// TODO you can ignore 'deny', because most of the time it allows payment retries
		// and later can become success
		SendMail("failed", transaction)
		h.TransactionRepository.UpdateTransaction("failed",  orderId)
	} else if transactionStatus == "cancel" || transactionStatus == "expire" {
		// TODO set transaction status on your databaase to 'failure'
		SendMail("failed", transaction)
		h.TransactionRepository.UpdateTransaction("failed",  orderId)
	} else if transactionStatus == "pending" {
		// TODO set transaction status on your databaase to 'pending' / waiting payment
		h.TransactionRepository.UpdateTransaction("pending",  orderId)
	}

	w.WriteHeader(http.StatusOK)
}

func convertResponseTransaction(t models.Transaction) transactiondto.TransactionResponse {
	return transactiondto.TransactionResponse{
		ID:      	t.ID,
		Product:   	t.Product,
		Buyer:  	t.Buyer,
		Seller: 	t.Seller,
		Price:  	t.Price,
		Status:    	t.Status,
	}
}

// function for handle send mail ...
func SendMail(status string, transaction models.Transaction) {

	if status != transaction.Status && (status == "success") {
		var CONFIG_SMTP_HOST = "smtp.gmail.com"
		var CONFIG_SMTP_PORT = 587
		var CONFIG_SENDER_NAME = "DumbMerch <demo.dumbways@gmail.com>"
		var CONFIG_AUTH_EMAIL = os.Getenv("EMAIL_SYSTEM")
		var CONFIG_AUTH_PASSWORD = os.Getenv("PASSWORD_SYSTEM")

		var productName = transaction.Product.Name
		var price = strconv.Itoa(transaction.Product.Price)

		mailer := gomail.NewMessage()
		mailer.SetHeader("From", CONFIG_SENDER_NAME)
		mailer.SetHeader("To", transaction.Buyer.Email)
		mailer.SetHeader("Subject", "Transaction Status")
		mailer.SetBody("text/html", fmt.Sprintf(`<!DOCTYPE html>
		<html lang="en">
		  <head>
			<meta charset="UTF-8" />
			<meta http-equiv="X-UA-Compatible" content="IE=edge" />
			<meta name="viewport" content="width=device-width, initial-scale=1.0" />
			<title>Document</title>
			<style>
			  h1 {
				color: brown;
			  }
			</style>
		  </head>
		  <body>
			<h2>Product payment :</h2>
			<ul style="list-style-type:none;">
			  <li>Name : %s</li>
			  <li>Total payment: Rp.%s</li>
			  <li>Status : <b>%s</b></li>
			</ul>  
		  </body>
		</html>`, productName, price, status))

		dialer := gomail.NewDialer(
			CONFIG_SMTP_HOST,
			CONFIG_SMTP_PORT,
			CONFIG_AUTH_EMAIL,
			CONFIG_AUTH_PASSWORD,
		)

		err := dialer.DialAndSend(mailer)
		if err != nil {
			log.Fatal(err.Error())
		}

		log.Println("===================================== SendMail =========================================")
		log.Println("Mail sent! to " + transaction.Buyer.Email)
		log.Println("========================================================================================")
	}
}