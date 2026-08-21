// Первичное наполнение базы.
//
// Отрабатывает только на пустой базе: категории заводятся, если их нет,
// изделия — если в каталоге пусто. Повторные запуски ничего не трогают,
// поэтому правки администратора никогда не затираются.
package main

// seedCategory — раздел каталога из поставки.
type seedCategory struct {
	Name  string
	Slug  string
	Order int
}

// Слаги совпадают с адресами в меню сайта (catalog.html?cat=kitchen),
// поэтому старые ссылки продолжают работать.
var seedCategories = []seedCategory{
	{"Диваны", "sofa", 10},
	{"Кресла", "armchair", 20},
	{"Кровати", "bed", 30},
	{"Шкафы", "wardrobe", 40},
	{"Столы", "table", 50},
	{"Стулья", "chair", 60},
	{"Комоды", "dresser", 70},
	{"Тумбы", "nightstand", 80},
	{"Кухонная мебель", "kitchen", 90},
	{"Офисная мебель", "office", 100},
	{"Лестницы", "stairs", 110},
	{"Двери и порталы", "doors", 120},
	{"Стеновые панели", "panels", 130},
	{"Мебель для ванной", "bath", 140},
	{"Другая мебель", "other", 150},
}

// seedProduct — изделие из поставки. Цены указаны в тенге и являются
// демонстрационными: администратор правит их в панели управления.
type seedProduct struct {
	Title        string
	Description  string
	Price        int64
	Image        string
	CategorySlug string
	Wood         string
	Badge        string
}

var seedProducts = []seedProduct{
	{
		Title:        "Диван «Комфорт»",
		Description:  "Современный мягкий диван для гостиной. Прочная конструкция на каркасе из массива бука, качественная обивка и удобные подушки со съёмными чехлами.",
		Price:        250000,
		Image:        "/assets/img/scenes/office.svg",
		CategorySlug: "sofa",
		Wood:         "ash",
		Badge:        "Хит",
	},
	{
		Title:        "Кухня «Dubrava Line»",
		Description:  "Фасады из ламелей дуба, интегрированная ручка, столешница из слэба 40 мм. Собирается по вашим размерам, фурнитура Blum с доводчиками.",
		Price:        1590000,
		Image:        "/assets/img/scenes/kitchen.svg",
		CategorySlug: "kitchen",
		Wood:         "oak",
		Badge:        "Хит",
	},
	{
		Title:        "Кухня «Noce Soft»",
		Description:  "Орех американский, радиусные торцы, встроенная подсветка рабочей зоны. Матовое масло подчёркивает рисунок волокна и не боится влаги.",
		Price:        2265000,
		Image:        "/assets/img/scenes/kitchen.svg",
		CategorySlug: "kitchen",
		Wood:         "walnut",
		Badge:        "Премиум",
	},
	{
		Title:        "Гардеробная «Fraxa Open»",
		Description:  "Открытая система до потолка, ясень под тонировку, штанги и полки с подсветкой. Наполнение подбирается под гардероб и площадь комнаты.",
		Price:        925000,
		Image:        "/assets/img/scenes/wardrobe.svg",
		CategorySlug: "wardrobe",
		Wood:         "ash",
	},
	{
		Title:        "Шкаф-купе «Larix»",
		Description:  "Лиственница, реечные фасады, доводчики Blum, глубина 600 мм. Практичный вариант для прихожей и спальни без потери площади.",
		Price:        615000,
		Image:        "/assets/img/scenes/wardrobe.svg",
		CategorySlug: "wardrobe",
		Wood:         "larch",
		Badge:        "Выгодно",
	},
	{
		Title:        "Лестница «Turn 180»",
		Description:  "Поворот на 180°, дуб на металлокаркасе, 16 ступеней, перила из массива. Замер, изготовление и монтаж — под ключ.",
		Price:        1870000,
		Image:        "/assets/img/scenes/stairs.svg",
		CategorySlug: "stairs",
		Wood:         "oak",
	},
	{
		Title:        "Лестница «Straight»",
		Description:  "Прямой марш на косоуре, ясень, подступенок белого цвета, 14 ступеней. Лаконичное решение для второго света и компактных холлов.",
		Price:        1475000,
		Image:        "/assets/img/scenes/stairs.svg",
		CategorySlug: "stairs",
		Wood:         "ash",
	},
	{
		Title:        "Дверь «Portal Slim»",
		Description:  "Скрытый короб, шпон ореха в одну карту, высота до 3 200 мм. Полотно заподлицо со стеной, петли и защёлка скрытого монтажа.",
		Price:        255000,
		Image:        "/assets/img/scenes/door.svg",
		CategorySlug: "doors",
		Wood:         "walnut",
	},
	{
		Title:        "Дверь «Flat Oak»",
		Description:  "Гладкое полотно из массива дуба, магнитная защёлка, кромка алюминий. Тонировка подбирается под пол и стеновые панели.",
		Price:        214000,
		Image:        "/assets/img/scenes/door.svg",
		CategorySlug: "doors",
		Wood:         "oak",
	},
	{
		Title:        "Реечная панель «Larix Rib»",
		Description:  "Рейка 40×20 мм на войлочной основе, монтаж на клипсы, шаг 15 мм. Цена указана за квадратный метр готовой панели.",
		Price:        27000,
		Image:        "/assets/img/scenes/panels.svg",
		CategorySlug: "panels",
		Wood:         "larch",
		Badge:        "Хит",
	},
	{
		Title:        "3D-панель «Wave»",
		Description:  "Волновая фрезеровка по ЧПУ, ясень, масло Rubio Monocoat. Рельеф собирается в непрерывный рисунок на всю стену.",
		Price:        41000,
		Image:        "/assets/img/scenes/panels.svg",
		CategorySlug: "panels",
		Wood:         "ash",
		Badge:        "Новинка",
	},
	{
		Title:        "Стол «Slab 2800»",
		Description:  "Слэб дуба 2 800 × 1 100 мм, эпоксидная заливка трещин, опоры-лофт. Каждая столешница уникальна по рисунку и форме кромки.",
		Price:        1023000,
		Image:        "/assets/img/scenes/table.svg",
		CategorySlug: "table",
		Wood:         "oak",
	},
	{
		Title:        "Кровать «Noce Bed»",
		Description:  "Массив ореха, парящее основание, изголовье 3,2 м, подъёмный механизм. Ортопедическое основание и ниша для белья входят в стоимость.",
		Price:        847000,
		Image:        "/assets/img/scenes/bed.svg",
		CategorySlug: "bed",
		Wood:         "walnut",
	},
	{
		Title:        "Тумба «Terma Bath»",
		Description:  "Термоясень, влагостойкое покрытие Osmo, накладная раковина. Термообработка защищает древесину от влаги ванной комнаты.",
		Price:        484000,
		Image:        "/assets/img/scenes/bath.svg",
		CategorySlug: "bath",
		Wood:         "thermo",
	},
	{
		Title:        "Комод «Wenge Line»",
		Description:  "Венге, 4 ящика с доводчиками, фрезерованный профиль вместо ручек. Глубина 450 мм — помещается даже в узкой спальне.",
		Price:        528000,
		Image:        "/assets/img/scenes/office.svg",
		CategorySlug: "dresser",
		Wood:         "wenge",
	},
}

// Seed заводит категории и демонстрационные изделия на пустой базе.
func (s *Store) Seed() error {
	cats, err := s.ListCategories()
	if err != nil {
		return err
	}

	if len(cats) == 0 {
		ts := now()
		for _, c := range seedCategories {
			if _, err := s.db.Exec(
				`INSERT INTO categories (name, slug, sort_order, created_at) VALUES (?, ?, ?, ?)`,
				c.Name, c.Slug, c.Order, ts); err != nil {
				return err
			}
		}
		logInfo("создано категорий: %d", len(seedCategories))

		if cats, err = s.ListCategories(); err != nil {
			return err
		}
	}

	var products int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM products`).Scan(&products); err != nil {
		return err
	}
	if products > 0 {
		return nil
	}

	bySlug := make(map[string]int64, len(cats))
	for _, c := range cats {
		bySlug[c.Slug] = c.ID
	}

	for _, p := range seedProducts {
		var catID *int64
		if id, ok := bySlug[p.CategorySlug]; ok {
			catID = &id
		}
		if _, err := s.CreateProduct(ProductInput{
			Title:       p.Title,
			Description: p.Description,
			Price:       p.Price,
			ImageURL:    p.Image,
			CategoryID:  catID,
			Status:      StatusActive,
			Wood:        p.Wood,
			Badge:       p.Badge,
		}); err != nil {
			return err
		}
	}
	logInfo("создано изделий: %d", len(seedProducts))
	return nil
}
