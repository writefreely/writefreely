CREATE TABLE IF NOT EXISTS "accesstokens" (
	"token" BYTEA PRIMARY KEY,
	"user_id" INT NOT NULL,
	"sudo" BOOLEAN NOT NULL DEFAULT FALSE,
	"one_time" BOOLEAN NOT NULL DEFAULT FALSE,
	"created" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
	"expires" TIMESTAMP DEFAULT NULL,
	"user_agent" TEXT DEFAULT NULL
);


CREATE TABLE IF NOT EXISTS "appcontent" (
	"id" TEXT PRIMARY KEY,
	"content" TEXT NOT NULL,
	"updated" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);


CREATE TABLE "appmigrations" (
	"version" INT NOT NULL,
	"migrated" TIMESTAMP NOT NULL,
	"result" TEXT NOT NULL
);


CREATE TABLE IF NOT EXISTS "collectionattributes" (
	"collection_id" INT NOT NULL,
	"attribute" TEXT NOT NULL,
	"value" TEXT NOT NULL,
	PRIMARY KEY ("collection_id", "attribute")
);


CREATE TABLE IF NOT EXISTS "collectionkeys" (
	"collection_id" INT PRIMARY KEY,
	"public_key" BYTEA NOT NULL,
	"private_key" BYTEA NOT NULL
);


CREATE TABLE IF NOT EXISTS "collectionpasswords" (
    "collection_id" INT PRIMARY KEY,
    "password" TEXT NOT NULL
);


CREATE TABLE IF NOT EXISTS "collectionredirects" (
    "prev_alias" TEXT NOT NULL PRIMARY KEY,
    "new_alias" TEXT NOT NULL
);


CREATE TABLE IF NOT EXISTS "collections" (
    "id" SERIAL PRIMARY KEY,
    "alias" TEXT DEFAULT NULL,
    "title" TEXT NOT NULL,
    "description" TEXT NOT NULL,
    "style_sheet" TEXT,
    "script" TEXT DEFAULT NULL,
    "format" TEXT DEFAULT NULL,
    "privacy" INT NOT NULL,
    "owner_id" INT NOT NULL,
    "view_count" INT NOT NULL
);


CREATE TABLE IF NOT EXISTS "posts" (
    "id" TEXT PRIMARY KEY,
    "slug" TEXT DEFAULT NULL,
    "modify_token" TEXT DEFAULT NULL,
    "text_appearance" TEXT NOT NULL DEFAULT 'norm',
    "language" TEXT DEFAULT NULL,
    "rtl" BOOLEAN DEFAULT NULL,
    "privacy" INT NOT NULL,
    "owner_id" INT DEFAULT NULL,
    "collection_id" INT DEFAULT NULL,
    "pinned_position" SMALLINT NULL,
    "created" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "updated" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
    "view_count" INT NOT NULL,
    "title" TEXT NOT NULL,
    "content" TEXT NOT NULL
);


CREATE TABLE IF NOT EXISTS "remotefollows" (
    "collection_id" INT NOT NULL,
    "remote_user_id" INT NOT NULL,
    "created" TIMESTAMP NOT NULL,
    PRIMARY KEY ("collection_id", "remote_user_id")
);


CREATE TABLE IF NOT EXISTS "remoteuserkeys" (
    "id" TEXT PRIMARY KEY,
    "remote_user_id" INT NOT NULL,
    "public_key" BYTEA NOT NULL
);


CREATE TABLE IF NOT EXISTS "remoteusers" (
    "id" SERIAL PRIMARY KEY,
    "actor_id" TEXT NOT NULL,
    "inbox" TEXT NOT NULL,
    "shared_inbox" TEXT NOT NULL
);


CREATE TABLE IF NOT EXISTS "userattributes" (
    "user_id" INT NOT NULL,
    "attribute" TEXT NOT NULL,
    "value" TEXT NOT NULL,
    PRIMARY KEY ("user_id", "attribute")
);


CREATE TABLE "userinvites" (
    "id" TEXT NOT NULL,
    "owner_id" INT NOT NULL,
    "max_uses" SMALLINT DEFAULT NULL,
    "created" TIMESTAMP NOT NULL,
    "expires" TIMESTAMP DEFAULT NULL,
    "inactive" BOOLEAN NOT NULL
);


CREATE TABLE "users" (
    "id" SERIAL PRIMARY KEY,
    "username" TEXT NOT NULL,
    "password" TEXT NOT NULL,
    "email" TEXT DEFAULT NULL,
    "created" TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);


CREATE TABLE "usersinvited" (
    "invite_id" TEXT NOT NULL,
    "user_id" INT NOT NULL
);


CREATE INDEX IF NOT EXISTS "alias" ON "collections" ("alias");
CREATE UNIQUE INDEX IF NOT EXISTS "id_slug" ON "posts" ("collection_id", "slug");
CREATE UNIQUE INDEX IF NOT EXISTS "owner_id" ON "posts" ("owner_id", "id");
CREATE INDEX IF NOT EXISTS "privacy_id" ON "posts" ("privacy", "id");
CREATE UNIQUE INDEX IF NOT EXISTS "remote_user_id" ON "remoteuserkeys" ("remote_user_id");
CREATE UNIQUE INDEX IF NOT EXISTS "actor_id" ON "remoteusers" ("actor_id");
CREATE UNIQUE INDEX IF NOT EXISTS "username" ON "users" ("username");
