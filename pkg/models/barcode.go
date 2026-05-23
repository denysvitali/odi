package models

import swiss_qr_code "github.com/denysvitali/go-swiss-qr-bill"

type Barcode struct {
	QRBill *swiss_qr_code.QrCode `json:"qr_bill"`
	Text   string                `json:"text"`
}
