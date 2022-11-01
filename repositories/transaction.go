package repositories

import (
	"dumbmerch/models"

	"gorm.io/gorm"
)

type TransactionRepository interface {
	FindTransactions(ID int) ([]models.Transaction, error)
	GetTransaction(ID int) (models.Transaction, error)
	GetOneTransaction(ID string) (models.Transaction, error) // Declare GetOneTransaction repository method ...
	CreateTransaction(transactions models.Transaction) (models.Transaction, error)
	UpdateTransaction(status string, ID string) (error) // Declare UpdateTransaction repository method ...
}

func RepositoryTransaction(db *gorm.DB) *repository {
	return &repository{db}
}

func (r *repository) FindTransactions(ID int) ([]models.Transaction, error) {
	var transactions []models.Transaction
	err := r.db.Preload("Product").Preload("Product.User").Preload("Buyer").Preload("Seller").Find(&transactions, "buyer_id = ?", ID).Error

	return transactions, err
}

func (r *repository) GetTransaction(ID int) (models.Transaction, error) {
	var transactions models.Transaction
	err := r.db.Preload("Product").Preload("Product.User").Preload("Buyer").Preload("Seller").Find(&transactions, "id = ?", ID).Error

	return transactions, err
}

// Create GetOneTransaction method ...
func (r *repository) GetOneTransaction(ID string) (models.Transaction, error) {
	var transaction models.Transaction
	err := r.db.Preload("Product").Preload("Product.User").Preload("Buyer").Preload("Seller").First(&transaction, "id = ?", ID).Error

	return transaction, err
}

func (r *repository) CreateTransaction(transactions models.Transaction) (models.Transaction, error) {
	err := r.db.Preload("Product").Preload("Product.User").Preload("Buyer").Preload("Seller").Create(&transactions).Error

	return transactions, err
}

// Create UpdateTransaction method ...
func (r *repository) UpdateTransaction(status string, ID string) (error) {
	var transaction models.Transaction
	r.db.Preload("Product").First(&transaction, ID)

	// If is different & Status is "success" decrement product quantity
	if status != transaction.Status && status == "success" {
		var product models.Product
		r.db.First(&product, transaction.Product.ID)
		product.Qty = product.Qty - 1
		r.db.Save(&product)
	}

	transaction.Status = status

	err := r.db.Save(&transaction).Error

	return err
}