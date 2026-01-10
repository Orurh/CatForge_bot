package domain

// Power — базовая оценка боевой силы кота.
// Чистая функция; доменная формула (можно менять по балансу).
func Power(c *Cat) int {
	return (c.ATKBase * 3) + (c.DEFBase * 2) + (c.HPBase / 2) + (c.SPDBase * 1) + (c.Level * 5)
}
