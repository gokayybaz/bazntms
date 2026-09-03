package geoip

// countryCentroids, ISO 3166-1 alpha-2 → yaklaşık ülke merkezi (enlem, boylam)
// + Türkçe ad. GeoIP haritası bir dünya haritası üzerine ülke bazında trafik
// balonu koyar; kesin sınır poligonu yerine merkez noktası yeter. Liste
// gerçek internet trafiğinin geldiği ülkeleri kapsayacak kadar geniş; eksik
// bir kod gelirse o ülke haritada gösterilmez (API yine sayar).
type Centroid struct {
	Lat, Lon float64
	Name     string
}

var countryCentroids = map[string]Centroid{
	"US": {39.8, -98.6, "ABD"}, "CA": {56.1, -106.3, "Kanada"}, "MX": {23.6, -102.5, "Meksika"},
	"BR": {-14.2, -51.9, "Brezilya"}, "AR": {-38.4, -63.6, "Arjantin"}, "CL": {-35.7, -71.5, "Şili"},
	"CO": {4.6, -74.3, "Kolombiya"}, "PE": {-9.2, -75.0, "Peru"}, "VE": {6.4, -66.6, "Venezuela"},
	"GB": {55.4, -3.4, "Birleşik Krallık"}, "IE": {53.4, -8.2, "İrlanda"}, "FR": {46.2, 2.2, "Fransa"},
	"DE": {51.2, 10.4, "Almanya"}, "NL": {52.1, 5.3, "Hollanda"}, "BE": {50.5, 4.5, "Belçika"},
	"LU": {49.8, 6.1, "Lüksemburg"}, "ES": {40.5, -3.7, "İspanya"}, "PT": {39.4, -8.2, "Portekiz"},
	"IT": {41.9, 12.6, "İtalya"}, "CH": {46.8, 8.2, "İsviçre"}, "AT": {47.5, 14.6, "Avusturya"},
	"SE": {60.1, 18.6, "İsveç"}, "NO": {60.5, 8.5, "Norveç"}, "FI": {61.9, 25.7, "Finlandiya"},
	"DK": {56.3, 9.5, "Danimarka"}, "PL": {51.9, 19.1, "Polonya"}, "CZ": {49.8, 15.5, "Çekya"},
	"SK": {48.7, 19.7, "Slovakya"}, "HU": {47.2, 19.5, "Macaristan"}, "RO": {45.9, 24.9, "Romanya"},
	"BG": {42.7, 25.5, "Bulgaristan"}, "GR": {39.1, 21.8, "Yunanistan"}, "RS": {44.0, 21.0, "Sırbistan"},
	"HR": {45.1, 15.2, "Hırvatistan"}, "SI": {46.1, 14.8, "Slovenya"}, "UA": {48.4, 31.2, "Ukrayna"},
	"BY": {53.7, 27.9, "Belarus"}, "LT": {55.2, 23.9, "Litvanya"}, "LV": {56.9, 24.6, "Letonya"},
	"EE": {58.6, 25.0, "Estonya"}, "RU": {61.5, 105.3, "Rusya"}, "IS": {65.0, -19.0, "İzlanda"},
	"TR": {39.0, 35.2, "Türkiye"}, "CY": {35.1, 33.4, "Kıbrıs"}, "IL": {31.0, 34.9, "İsrail"},
	"SA": {23.9, 45.1, "Suudi Arabistan"}, "AE": {23.4, 53.8, "BAE"}, "QA": {25.4, 51.2, "Katar"},
	"KW": {29.3, 47.5, "Kuveyt"}, "IR": {32.4, 53.7, "İran"}, "IQ": {33.2, 43.7, "Irak"},
	"JO": {30.6, 36.2, "Ürdün"}, "LB": {33.9, 35.9, "Lübnan"}, "EG": {26.8, 30.8, "Mısır"},
	"MA": {31.8, -7.1, "Fas"}, "DZ": {28.0, 1.7, "Cezayir"}, "TN": {33.9, 9.6, "Tunus"},
	"LY": {26.3, 17.2, "Libya"}, "NG": {9.1, 8.7, "Nijerya"}, "ZA": {-30.6, 22.9, "Güney Afrika"},
	"KE": {-0.0, 37.9, "Kenya"}, "ET": {9.1, 40.5, "Etiyopya"}, "GH": {7.9, -1.0, "Gana"},
	"CI": {7.5, -5.5, "Fildişi Sahili"}, "SN": {14.5, -14.5, "Senegal"}, "TZ": {-6.4, 34.9, "Tanzanya"},
	"UG": {1.4, 32.3, "Uganda"}, "AO": {-11.2, 17.9, "Angola"},
	"IN": {20.6, 79.0, "Hindistan"}, "PK": {30.4, 69.3, "Pakistan"}, "BD": {23.7, 90.4, "Bangladeş"},
	"LK": {7.9, 80.8, "Sri Lanka"}, "NP": {28.4, 84.1, "Nepal"}, "CN": {35.9, 104.2, "Çin"},
	"HK": {22.3, 114.2, "Hong Kong"}, "TW": {23.7, 121.0, "Tayvan"}, "JP": {36.2, 138.3, "Japonya"},
	"KR": {35.9, 127.8, "Güney Kore"}, "MN": {46.9, 103.8, "Moğolistan"}, "KZ": {48.0, 66.9, "Kazakistan"},
	"UZ": {41.4, 64.6, "Özbekistan"}, "AZ": {40.1, 47.6, "Azerbaycan"}, "GE": {42.3, 43.4, "Gürcistan"},
	"AM": {40.1, 45.0, "Ermenistan"}, "TH": {15.9, 100.9, "Tayland"}, "VN": {14.1, 108.3, "Vietnam"},
	"MY": {4.2, 101.9, "Malezya"}, "SG": {1.35, 103.8, "Singapur"}, "ID": {-0.8, 113.9, "Endonezya"},
	"PH": {12.9, 121.8, "Filipinler"}, "MM": {21.9, 95.9, "Myanmar"}, "KH": {12.6, 104.9, "Kamboçya"},
	"AU": {-25.3, 133.8, "Avustralya"}, "NZ": {-40.9, 174.9, "Yeni Zelanda"}, "FJ": {-17.7, 178.1, "Fiji"},
}

// Centroid, ülke kodunun merkezini döndürür. Bulunamazsa ok=false.
func CountryCentroid(iso2 string) (Centroid, bool) {
	c, ok := countryCentroids[iso2]
	return c, ok
}
