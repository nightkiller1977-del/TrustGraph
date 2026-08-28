-- Phase 1 Plane B: widen subject_education.grade
--
-- Migration 002 declared this VARCHAR(10), sized for numeric GPAs like
-- "3.8". The Go model exposes an unrestricted string, and real values
-- include textual classifications like "First Class Honours" or "Summa
-- Cum Laude" that don't fit. RunMigrations tracks applied versions in
-- schema_migrations and skips a file whose version is already recorded,
-- so a database that already applied 002 would never re-run an edit made
-- directly to that file — this has to be a new migration, not a change to
-- 002's original CREATE TABLE statement.

ALTER TABLE subject_education ALTER COLUMN grade TYPE TEXT;
