CREATE TABLE "waktu_konsultasi" (
  "id" bigserial PRIMARY KEY,
  "psikolog_id" bigint NOT NULL,
  "hari" varchar(20) NOT NULL,
  "waktu_mulai" time NOT NULL,
  "waktu_selesai" time NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT (now()),
  
  -- Menambahkan foreign key constraint ke tabel users
  CONSTRAINT fk_psikolog
    FOREIGN KEY("psikolog_id") 
    REFERENCES "users"("id")
    ON DELETE CASCADE
);

-- Menambahkan indeks untuk mempercepat pencarian berdasarkan psikolog_id
CREATE INDEX ON "waktu_konsultasi" ("psikolog_id");
