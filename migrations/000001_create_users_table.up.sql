-- Membuat tipe data ENUM untuk peran pengguna
CREATE TYPE user_role AS ENUM ('admin', 'psikolog', 'klien');

-- Membuat tabel users
CREATE TABLE "users" (
  "id" bigserial PRIMARY KEY,
  "username" varchar(100) NOT NULL,
  "email" varchar(100) UNIQUE NOT NULL,
  "password" varchar(255) NOT NULL,
  "phone_number" varchar(20),
  "gender" varchar(10),
  "profile_picture" varchar(255),
  "role" user_role NOT NULL,
  "created_at" timestamptz NOT NULL DEFAULT (now())
);

-- Menambahkan indeks pada email untuk pencarian yang lebih cepat
CREATE INDEX ON "users" ("email");
