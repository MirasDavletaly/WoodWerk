// Первичное наполнение базы.
//
// Отрабатывает только на пустой базе: категории заводятся, если их нет,
// изделия — если в каталоге пусто. Повторные запуски ничего не трогают,
// поэтому правки администратора никогда не затираются.
package main

// seedCategory — раздел каталога из поставки.
type seedCategory struct {
	Name   string
	NameKK string
	NameEN string
	Slug   string
	Order  int
}

// Слаги совпадают с адресами в меню сайта (catalog.html?cat=kitchen),
// поэтому старые ссылки продолжают работать.
var seedCategories = []seedCategory{
	{"Диваны", "Дивандар", "Sofas", "sofa", 10},
	{"Кресла", "Креслолар", "Armchairs", "armchair", 20},
	{"Кровати", "Төсектер", "Beds", "bed", 30},
	{"Шкафы", "Шкафтар", "Wardrobes", "wardrobe", 40},
	{"Столы", "Үстелдер", "Tables", "table", 50},
	{"Стулья", "Орындықтар", "Chairs", "chair", 60},
	{"Комоды", "Комодтар", "Chests of drawers", "dresser", 70},
	{"Тумбы", "Тумбалар", "Cabinets", "nightstand", 80},
	{"Кухонная мебель", "Ас үй жиһазы", "Kitchen furniture", "kitchen", 90},
	{"Офисная мебель", "Кеңсе жиһазы", "Office furniture", "office", 100},
	{"Лестницы", "Баспалдақтар", "Staircases", "stairs", 110},
	{"Двери и порталы", "Есіктер мен порталдар", "Doors and portals", "doors", 120},
	{"Стеновые панели", "Қабырға панельдері", "Wall panels", "panels", 130},
	{"Мебель для ванной", "Жуынатын бөлме жиһазы", "Bathroom furniture", "bath", 140},
	{"Другая мебель", "Басқа жиһаз", "Other furniture", "other", 150},
}

// seedProduct — изделие из поставки. Цены указаны в тенге и являются
// демонстрационными: администратор правит их в панели управления.
type seedProduct struct {
	Title         string
	TitleKK       string
	TitleEN       string
	Description   string
	DescriptionKK string
	DescriptionEN string
	Price         int64
	Image         string
	CategorySlug  string
	Wood          string
	Badge         string
}

var seedProducts = []seedProduct{
	{
		Title:         "Диван «Комфорт»",
		TitleKK:       "«Комфорт» диваны",
		TitleEN:       "Comfort sofa",
		Description:   "Современный мягкий диван для гостиной. Прочная конструкция на каркасе из массива бука, качественная обивка и удобные подушки со съёмными чехлами.",
		DescriptionKK: "Қонақ бөлмеге арналған заманауи жұмсақ диван. Шамшат массивінің қаңқасындағы берік құрылым, сапалы қаптама және алмалы-салмалы тысы бар ыңғайлы жастықтар.",
		DescriptionEN: "A modern upholstered sofa for the living room. A sturdy solid beech frame, quality upholstery and comfortable cushions with removable covers.",
		Price:         250000,
		Image:         "/assets/img/scenes/office.svg",
		CategorySlug:  "sofa",
		Wood:          "ash",
		Badge:         "Хит",
	},
	{
		Title:         "Кухня «Dubrava Line»",
		TitleKK:       "«Dubrava Line» ас үйі",
		TitleEN:       "Dubrava Line kitchen",
		Description:   "Фасады из ламелей дуба, интегрированная ручка, столешница из слэба 40 мм. Собирается по вашим размерам, фурнитура Blum с доводчиками.",
		DescriptionKK: "Емен ламельдерінен жасалған қаптамалар, кірістірілген тұтқа, 40 мм слэбтен үстел үсті. Сіздің өлшеміңіз бойынша жиналады, жабылу тетіктері бар Blum фурнитурасы.",
		DescriptionEN: "Oak-slat fronts, an integrated handle and a 40 mm slab worktop. Built to your dimensions with Blum soft-close hardware.",
		Price:         1590000,
		Image:         "/assets/img/scenes/kitchen.svg",
		CategorySlug:  "kitchen",
		Wood:          "oak",
		Badge:         "Хит",
	},
	{
		Title:         "Кухня «Noce Soft»",
		TitleKK:       "«Noce Soft» ас үйі",
		TitleEN:       "Noce Soft kitchen",
		Description:   "Орех американский, радиусные торцы, встроенная подсветка рабочей зоны. Матовое масло подчёркивает рисунок волокна и не боится влаги.",
		DescriptionKK: "Американдық жаңғақ ағашы, радиусты шеттер, жұмыс аймағының кірістірілген жарығы. Күңгірт май талшық өрнегін айқындайды және ылғалдан қорықпайды.",
		DescriptionEN: "American walnut, radiused edges and integrated task lighting. A matt oil brings out the grain and shrugs off moisture.",
		Price:         2265000,
		Image:         "/assets/img/scenes/kitchen.svg",
		CategorySlug:  "kitchen",
		Wood:          "walnut",
		Badge:         "Премиум",
	},
	{
		Title:         "Гардеробная «Fraxa Open»",
		TitleKK:       "«Fraxa Open» гардеробы",
		TitleEN:       "Fraxa Open dressing room",
		Description:   "Открытая система до потолка, ясень под тонировку, штанги и полки с подсветкой. Наполнение подбирается под гардероб и площадь комнаты.",
		DescriptionKK: "Төбеге дейінгі ашық жүйе, тонировкаға арналған шаған, жарығы бар ілгіштер мен сөрелер. Толтырылуы гардеробқа және бөлме ауданына қарай таңдалады.",
		DescriptionEN: "An open floor-to-ceiling system in ash ready for tinting, with lit rails and shelves. The internal layout is planned around your wardrobe and the size of the room.",
		Price:         925000,
		Image:         "/assets/img/scenes/wardrobe.svg",
		CategorySlug:  "wardrobe",
		Wood:          "ash",
	},
	{
		Title:         "Шкаф-купе «Larix»",
		TitleKK:       "«Larix» купе шкафы",
		TitleEN:       "Larix sliding wardrobe",
		Description:   "Лиственница, реечные фасады, доводчики Blum, глубина 600 мм. Практичный вариант для прихожей и спальни без потери площади.",
		DescriptionKK: "Балқарағай, рейкалы қаптамалар, Blum жабылу тетіктері, тереңдігі 600 мм. Дәліз бен жатын бөлмеге аудан жоғалтпайтын тиімді нұсқа.",
		DescriptionEN: "Larch with slatted fronts, Blum soft-close hardware and a 600 mm depth. A practical choice for a hallway or bedroom that doesn't eat floor space.",
		Price:         615000,
		Image:         "/assets/img/scenes/wardrobe.svg",
		CategorySlug:  "wardrobe",
		Wood:          "larch",
		Badge:         "Выгодно",
	},
	{
		Title:         "Лестница «Turn 180»",
		TitleKK:       "«Turn 180» баспалдағы",
		TitleEN:       "Turn 180 staircase",
		Description:   "Поворот на 180°, дуб на металлокаркасе, 16 ступеней, перила из массива. Замер, изготовление и монтаж — под ключ.",
		DescriptionKK: "180° бұрылыс, метал қаңқадағы емен, 16 баспалдақ, массивтен жасалған тұтқа. Өлшеу, дайындау және орнату — кілт тапсырымымен.",
		DescriptionEN: "A 180° turn, oak on a steel frame, 16 steps and a solid wood handrail. Measurement, manufacture and installation, turnkey.",
		Price:         1870000,
		Image:         "/assets/img/scenes/stairs.svg",
		CategorySlug:  "stairs",
		Wood:          "oak",
	},
	{
		Title:         "Лестница «Straight»",
		TitleKK:       "«Straight» баспалдағы",
		TitleEN:       "Straight staircase",
		Description:   "Прямой марш на косоуре, ясень, подступенок белого цвета, 14 ступеней. Лаконичное решение для второго света и компактных холлов.",
		DescriptionKK: "Қиғаш арқалықтағы тік марш, шаған, ақ түсті тік тақтай, 14 баспалдақ. Екінші жарық пен шағын холдарға арналған ықшам шешім.",
		DescriptionEN: "A straight flight on a stringer in ash with white risers and 14 steps. A restrained solution for double-height spaces and compact halls.",
		Price:         1475000,
		Image:         "/assets/img/scenes/stairs.svg",
		CategorySlug:  "stairs",
		Wood:          "ash",
	},
	{
		Title:         "Дверь «Portal Slim»",
		TitleKK:       "«Portal Slim» есігі",
		TitleEN:       "Portal Slim door",
		Description:   "Скрытый короб, шпон ореха в одну карту, высота до 3 200 мм. Полотно заподлицо со стеной, петли и защёлка скрытого монтажа.",
		DescriptionKK: "Жасырын жақтау, бір картадағы жаңғақ ағашының шпоны, биіктігі 3 200 мм-ге дейін. Жапырақ қабырғамен бірдей деңгейде, топсалар мен ілгек жасырын орнатылады.",
		DescriptionEN: "A concealed frame, single-sheet walnut veneer and heights up to 3,200 mm. The leaf sits flush with the wall, with concealed hinges and latch.",
		Price:         255000,
		Image:         "/assets/img/scenes/door.svg",
		CategorySlug:  "doors",
		Wood:          "walnut",
	},
	{
		Title:         "Дверь «Flat Oak»",
		TitleKK:       "«Flat Oak» есігі",
		TitleEN:       "Flat Oak door",
		Description:   "Гладкое полотно из массива дуба, магнитная защёлка, кромка алюминий. Тонировка подбирается под пол и стеновые панели.",
		DescriptionKK: "Емен массивінен жасалған тегіс жапырақ, магнитті ілгек, алюминий жиегі. Тонировка еден мен қабырға панельдеріне қарай таңдалады.",
		DescriptionEN: "A flat solid oak leaf with a magnetic latch and an aluminium edge. The tint is matched to your floor and wall panels.",
		Price:         214000,
		Image:         "/assets/img/scenes/door.svg",
		CategorySlug:  "doors",
		Wood:          "oak",
	},
	{
		Title:         "Реечная панель «Larix Rib»",
		TitleKK:       "«Larix Rib» рейкалы панелі",
		TitleEN:       "Larix Rib slatted panel",
		Description:   "Рейка 40×20 мм на войлочной основе, монтаж на клипсы, шаг 15 мм. Цена указана за квадратный метр готовой панели.",
		DescriptionKK: "Киіз негіздегі 40×20 мм рейка, клипсаға орнату, қадамы 15 мм. Баға дайын панельдің шаршы метрі үшін көрсетілген.",
		DescriptionEN: "A 40×20 mm slat on a felt backing, clip-on fixing, 15 mm spacing. The price is per square metre of finished panel.",
		Price:         27000,
		Image:         "/assets/img/scenes/panels.svg",
		CategorySlug:  "panels",
		Wood:          "larch",
		Badge:         "Хит",
	},
	{
		Title:         "3D-панель «Wave»",
		TitleKK:       "«Wave» 3D-панелі",
		TitleEN:       "Wave 3D panel",
		Description:   "Волновая фрезеровка по ЧПУ, ясень, масло Rubio Monocoat. Рельеф собирается в непрерывный рисунок на всю стену.",
		DescriptionKK: "ЧПУ-мен толқынды фрезерлеу, шаған, Rubio Monocoat майы. Бедер бүкіл қабырғаға үздіксіз өрнекке жиналады.",
		DescriptionEN: "CNC wave milling in ash, finished with Rubio Monocoat oil. The relief joins into a continuous pattern across the whole wall.",
		Price:         41000,
		Image:         "/assets/img/scenes/panels.svg",
		CategorySlug:  "panels",
		Wood:          "ash",
		Badge:         "Новинка",
	},
	{
		Title:         "Стол «Slab 2800»",
		TitleKK:       "«Slab 2800» үстелі",
		TitleEN:       "Slab 2800 table",
		Description:   "Слэб дуба 2 800 × 1 100 мм, эпоксидная заливка трещин, опоры-лофт. Каждая столешница уникальна по рисунку и форме кромки.",
		DescriptionKK: "2 800 × 1 100 мм емен слэбі, жарықтарды эпоксидпен құю, лофт тіректері. Әр үстел үсті өрнегі мен жиек пішіні бойынша бірегей.",
		DescriptionEN: "A 2,800 × 1,100 mm oak slab with epoxy-filled cracks and loft-style legs. Every top is unique in its figure and edge profile.",
		Price:         1023000,
		Image:         "/assets/img/scenes/table.svg",
		CategorySlug:  "table",
		Wood:          "oak",
	},
	{
		Title:         "Кровать «Noce Bed»",
		TitleKK:       "«Noce Bed» төсегі",
		TitleEN:       "Noce Bed",
		Description:   "Массив ореха, парящее основание, изголовье 3,2 м, подъёмный механизм. Ортопедическое основание и ниша для белья входят в стоимость.",
		DescriptionKK: "Жаңғақ ағашының массиві, қалқыма негіз, 3,2 м бас жағы, көтергіш механизм. Ортопедиялық негіз бен кір салатын қуыс құнға кіреді.",
		DescriptionEN: "Solid walnut with a floating base, a 3.2 m headboard and a lift mechanism. The slatted base and storage compartment are included.",
		Price:         847000,
		Image:         "/assets/img/scenes/bed.svg",
		CategorySlug:  "bed",
		Wood:          "walnut",
	},
	{
		Title:         "Тумба «Terma Bath»",
		TitleKK:       "«Terma Bath» тумбасы",
		TitleEN:       "Terma Bath vanity unit",
		Description:   "Термоясень, влагостойкое покрытие Osmo, накладная раковина. Термообработка защищает древесину от влаги ванной комнаты.",
		DescriptionKK: "Термошаған, ылғалға төзімді Osmo жабыны, үстіне қойылатын раковина. Термоөңдеу ағашты жуынатын бөлме ылғалынан қорғайды.",
		DescriptionEN: "Thermo-ash with a moisture-resistant Osmo finish and a countertop basin. Thermal treatment protects the wood from bathroom humidity.",
		Price:         484000,
		Image:         "/assets/img/scenes/bath.svg",
		CategorySlug:  "bath",
		Wood:          "thermo",
	},
	{
		Title:         "Комод «Wenge Line»",
		TitleKK:       "«Wenge Line» комоды",
		TitleEN:       "Wenge Line chest of drawers",
		Description:   "Венге, 4 ящика с доводчиками, фрезерованный профиль вместо ручек. Глубина 450 мм — помещается даже в узкой спальне.",
		DescriptionKK: "Венге, жабылу тетіктері бар 4 тартпа, тұтқаның орнына фрезерленген профиль. Тереңдігі 450 мм — тар жатын бөлмеге де сыяды.",
		DescriptionEN: "Wenge with four soft-close drawers and a milled profile instead of handles. At 450 mm deep it fits even a narrow bedroom.",
		Price:         528000,
		Image:         "/assets/img/scenes/office.svg",
		CategorySlug:  "dresser",
		Wood:          "wenge",
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
				`INSERT INTO categories (name, name_kk, name_en, slug, sort_order, created_at)
                 VALUES (?, ?, ?, ?, ?, ?)`,
				c.Name, c.NameKK, c.NameEN, c.Slug, c.Order, ts); err != nil {
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
			Title:         p.Title,
			TitleKK:       p.TitleKK,
			TitleEN:       p.TitleEN,
			Description:   p.Description,
			DescriptionKK: p.DescriptionKK,
			DescriptionEN: p.DescriptionEN,
			Price:         p.Price,
			ImageURL:      p.Image,
			CategoryID:    catID,
			Status:        StatusActive,
			Wood:          p.Wood,
			Badge:         p.Badge,
		}); err != nil {
			return err
		}
	}
	logInfo("создано изделий: %d", len(seedProducts))
	return nil
}
