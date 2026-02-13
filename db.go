package main

import (
	"database/sql"
	"fmt"

	_ "github.com/mattn/go-sqlite3"
)

var db *sql.DB

func initDB(path string) error {
	var err error
	db, err = sql.Open("sqlite3", path)
	if err != nil {
		return fmt.Errorf("error abriendo BD: %w", err)
	}

	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS estudiantes (
		ci TEXT PRIMARY KEY,
		pin_encrypted TEXT NOT NULL,
		inscripcion INTEGER DEFAULT 0,
		tipo INTEGER DEFAULT 0,
		carrera TEXT DEFAULT ''
	)`)
	if err != nil {
		return fmt.Errorf("error creando tabla: %w", err)
	}

	// Migrar BD existente que no tenga las columnas de cache
	db.Exec(`ALTER TABLE estudiantes ADD COLUMN inscripcion INTEGER DEFAULT 0`)
	db.Exec(`ALTER TABLE estudiantes ADD COLUMN tipo INTEGER DEFAULT 0`)
	db.Exec(`ALTER TABLE estudiantes ADD COLUMN carrera TEXT DEFAULT ''`)

	return nil
}

func guardarEstudiante(ci, pinEncrypted string) error {
	_, err := db.Exec(
		`INSERT OR REPLACE INTO estudiantes (ci, pin_encrypted) VALUES (?, ?)`,
		ci, pinEncrypted,
	)
	return err
}

func obtenerPinEncriptado(ci string) (string, error) {
	var pinEnc string
	err := db.QueryRow(`SELECT pin_encrypted FROM estudiantes WHERE ci = ?`, ci).Scan(&pinEnc)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("CI %s no registrado", ci)
	}
	return pinEnc, err
}

func contarEstudiantes() (int, error) {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM estudiantes`).Scan(&count)
	return count, err
}

func obtenerCache(ci string) (inscripcion int, tipo int, carrera string, ok bool) {
	err := db.QueryRow(
		`SELECT inscripcion, tipo, carrera FROM estudiantes WHERE ci = ? AND inscripcion > 0`, ci,
	).Scan(&inscripcion, &tipo, &carrera)
	return inscripcion, tipo, carrera, err == nil
}

func guardarCache(ci string, inscripcion, tipo int, carrera string) {
	db.Exec(`UPDATE estudiantes SET inscripcion = ?, tipo = ?, carrera = ? WHERE ci = ?`,
		inscripcion, tipo, carrera, ci)
}
