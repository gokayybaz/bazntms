package geoip

import "testing"

func TestCountryCentroidSane(t *testing.T) {
	if _, ok := CountryCentroid("ZZ"); ok {
		t.Fatal("bilinmeyen kod ok=true dondurdu")
	}
	for iso, c := range countryCentroids {
		if len(iso) != 2 {
			t.Errorf("ISO2 olmayan anahtar: %q", iso)
		}
		if c.Lat < -90 || c.Lat > 90 || c.Lon < -180 || c.Lon > 180 {
			t.Errorf("%s: koordinat aralik disi (%v, %v)", iso, c.Lat, c.Lon)
		}
		if c.Name == "" {
			t.Errorf("%s: ad bos", iso)
		}
	}
	tr, ok := CountryCentroid("TR")
	if !ok || tr.Name != "Türkiye" {
		t.Fatalf("TR merkezi beklenirdi: %+v ok=%v", tr, ok)
	}
}
