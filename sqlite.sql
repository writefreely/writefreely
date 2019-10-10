--
-- Database: writefreely
--

-- --------------------------------------------------------

--
-- Table structure for table accesstokens
--

CREATE TABLE IF NOT EXISTS `accesstokens` (
  token TEXT NOT NULL PRIMARY KEY,
  user_id INTEGER NOT NULL,
  sudo INTEGER NOT NULL DEFAULT '0',
  one_time INTEGER NOT NULL DEFAULT '0',
  created DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  expires DATETIME DEFAULT NULL,
  user_agent TEXT DEFAULT NULL
);

-- --------------------------------------------------------

--
-- Table structure for table appcontent
--

CREATE TABLE IF NOT EXISTS `appcontent` (
  id TEXT NOT NULL PRIMARY KEY,
  content TEXT NOT NULL,
  updated DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- --------------------------------------------------------

--
-- Table structure for table appmigrations
--

CREATE TABLE `appmigrations` (
  `version` INT NOT NULL,
  `migrated` DATETIME NOT NULL,
  `result` TEXT NOT NULL
);

-- --------------------------------------------------------

--
-- Table structure for table collectionattributes
--

CREATE TABLE IF NOT EXISTS `collectionattributes` (
  collection_id INTEGER NOT NULL,
  attribute TEXT NOT NULL,
  value TEXT NOT NULL,
  PRIMARY KEY (collection_id, attribute)
);

-- --------------------------------------------------------

--
-- Table structure for table collectionkeys
--

CREATE TABLE IF NOT EXISTS `collectionkeys` (
  collection_id INTEGER PRIMARY KEY,
  public_key blob NOT NULL,
  private_key blob NOT NULL
);

-- --------------------------------------------------------

--
-- Table structure for table collectionpasswords
--

CREATE TABLE IF NOT EXISTS `collectionpasswords` (
  collection_id INTEGER PRIMARY KEY,
  password TEXT NOT NULL
);

-- --------------------------------------------------------

--
-- Table structure for table collectionredirects
--

CREATE TABLE IF NOT EXISTS `collectionredirects` (
  prev_alias TEXT NOT NULL PRIMARY KEY,
  new_alias TEXT NOT NULL
);

-- --------------------------------------------------------

--
-- Table structure for table collections
--

CREATE TABLE IF NOT EXISTS `collections` (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  alias TEXT DEFAULT NULL UNIQUE,
  title TEXT NOT NULL,
  description TEXT NOT NULL,
  style_sheet TEXT,
  script TEXT,
  format TEXT DEFAULT NULL,
  privacy INTEGER NOT NULL,
  owner_id INTEGER NOT NULL,
  view_count INTEGER NOT NULL
);

-- --------------------------------------------------------

--
-- Table structure for table posts
--

CREATE TABLE IF NOT EXISTS `posts` (
  id TEXT NOT NULL,
  slug TEXT DEFAULT NULL,
  modify_token TEXT DEFAULT NULL,
  text_appearance TEXT NOT NULL DEFAULT 'norm',
  language TEXT DEFAULT NULL,
  rtl INTEGER DEFAULT NULL,
  privacy INTEGER NOT NULL,
  owner_id INTEGER DEFAULT NULL,
  collection_id INTEGER DEFAULT NULL,
  pinned_position INTEGER UNSIGNED DEFAULT NULL,
  created DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  view_count INTEGER NOT NULL,
  title TEXT NOT NULL,
  content TEXT NOT NULL,
  CONSTRAINT id_slug UNIQUE (collection_id, slug),
  CONSTRAINT owner_id UNIQUE (owner_id, id),
  CONSTRAINT privacy_id UNIQUE (privacy, id)
);

-- --------------------------------------------------------

--
-- Table structure for table remotefollows
--

CREATE TABLE IF NOT EXISTS `remotefollows` (
  collection_id INTEGER NOT NULL,
  remote_user_id INTEGER NOT NULL,
  created DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (collection_id,remote_user_id)
);

-- --------------------------------------------------------

--
-- Table structure for table remoteuserkeys
--

CREATE TABLE IF NOT EXISTS `remoteuserkeys` (
  id TEXT NOT NULL,
  remote_user_id INTEGER NOT NULL,
  public_key blob NOT NULL,
  CONSTRAINT follower_id UNIQUE (remote_user_id)
);

-- --------------------------------------------------------

--
-- Table structure for table remoteusers
--

CREATE TABLE IF NOT EXISTS `remoteusers` (
  id INTEGER NOT NULL PRIMARY KEY AUTOINCREMENT,
  actor_id TEXT NOT NULL,
  inbox TEXT NOT NULL,
  shared_inbox TEXT NOT NULL,
  handle TEXT DEFAULT '' NOT NULL,
  CONSTRAINT collection_id UNIQUE (actor_id)
);

-- --------------------------------------------------------

--
-- Table structure for table userattributes
--

CREATE TABLE IF NOT EXISTS `userattributes` (
  user_id INTEGER NOT NULL,
  attribute TEXT NOT NULL,
  value TEXT NOT NULL,
  PRIMARY KEY (user_id, attribute)
);

-- --------------------------------------------------------

--
-- Table structure for table `userinvites`
--

CREATE TABLE `userinvites` (
  `id` TEXT NOT NULL,
  `owner_id` INTEGER NOT NULL,
  `max_uses` INTEGER DEFAULT NULL,
  `created` DATETIME NOT NULL,
  `expires` DATETIME DEFAULT NULL,
  `inactive` INTEGER NOT NULL
);

-- --------------------------------------------------------

--
-- Table structure for table users
--

CREATE TABLE IF NOT EXISTS `users` (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  username TEXT NOT NULL UNIQUE,
  password TEXT NOT NULL,
  email TEXT DEFAULT NULL,
  created DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- --------------------------------------------------------

--
-- Table structure for table `usersinvited`
--

CREATE TABLE `usersinvited` (
  `invite_id` TEXT NOT NULL,
  `user_id` INTEGER NOT NULL
);
