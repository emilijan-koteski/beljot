-- Reverse 000012 by dropping the private-room password column.
ALTER TABLE rooms DROP COLUMN password_hash;
