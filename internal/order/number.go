package order

// PassesLuhn проверяет непустой номер из ASCII-цифр по алгоритму Луна.
// Проверка контрольной суммы не подтверждает существование заказа.
func PassesLuhn(orderId string) bool {
	sum := 0
	double := false

	for i := len(orderId) - 1; i >= 0; i-- {
		digit := int(orderId[i]) - '0'
		if digit < 0 || digit > 9 {
			return false
		}

		if double {
			digit *= 2
			if digit > 9 {
				digit -= 9
			}
		}

		sum += digit
		double = !double
	}

	return len(orderId) > 0 && sum%10 == 0
}
