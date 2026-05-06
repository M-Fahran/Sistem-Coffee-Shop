package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"

	_ "github.com/lib/pq"
)

func main() {
	// 1. Cek apakah ada opsi "reset" saat file dijalankan
	isReset := len(os.Args) > 1 && os.Args[1] == "reset"

	fmt.Println("⏳ Menghubungkan ke database...")

	dsn := "host=localhost user=root password=123 dbname=coffeeshop_db port=5432 sslmode=disable"
	db, err := sql.Open("postgres", dsn)
	if err != nil {
		log.Fatalf("❌ Gagal inisiasi koneksi: %v\n", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		log.Fatalf("❌ Database tidak merespon: %v\n", err)
	}
	fmt.Println("✅ Database terhubung!")

	// Deteksi folder
	migrationsDir := "migration"
	if _, err := os.Stat(migrationsDir); os.IsNotExist(err) {
		migrationsDir = "."
	}

	// ==========================================
	// FITUR BARU: RESET (Menjalankan .down.sql)
	// ==========================================
	if isReset {
		fmt.Println("\n⚠️ MENGAKTIFKAN MODE RESET: Menghapus tabel lama...")
		
		downFiles, err := filepath.Glob(filepath.Join(migrationsDir, "*.down.sql"))
		if err == nil && len(downFiles) > 0 {
			// Urutkan dari belakang (Z-A) karena menghapus tabel harus dari ujung relasi
			sort.Sort(sort.Reverse(sort.StringSlice(downFiles)))
			
			for _, file := range downFiles {
				fmt.Printf("   🗑️ Mengeksekusi Drop: %s\n", filepath.Base(file))
				content, err := os.ReadFile(file)
				if err != nil {
					log.Fatalf("❌ Gagal membaca file %s: %v\n", file, err)
				}
				_, err = db.Exec(string(content))
				if err != nil {
					log.Fatalf("❌ Gagal melakukan DROP di %s: %v\n", file, err)
				}
			}
			fmt.Println("✅ Tabel lama berhasil dihapus!")
		} else {
			fmt.Println("⚠️ Tidak ada file .down.sql ditemukan, melewati proses penghapusan.")
		}
	}

	// ==========================================
	// FITUR MIGRASI NORMAL (Menjalankan .up.sql)
	// ==========================================
	fmt.Println("\n⚙️ Memulai proses pembuatan tabel...")
	upFiles, err := filepath.Glob(filepath.Join(migrationsDir, "*.up.sql"))
	if err != nil || len(upFiles) == 0 {
		log.Fatalf("❌ Tidak ada file .up.sql yang ditemukan di folder %s\n", migrationsDir)
	}

	sort.Strings(upFiles)

	for _, file := range upFiles {
		fmt.Printf("   🔨 Mengeksekusi Create: %s\n", filepath.Base(file))
		content, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("❌ Gagal membaca file %s: %v\n", file, err)
		}
		_, err = db.Exec(string(content))
		if err != nil {
			log.Fatalf("❌ Gagal mengeksekusi kueri di %s: %v\n", file, err)
		}
	}

	fmt.Println("\n🚀 MIGRATION SUCCESS! Database siap digunakan.")
}