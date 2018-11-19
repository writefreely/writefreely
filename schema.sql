--
-- Database: `writefreely`
--

-- --------------------------------------------------------

--
-- Table structure for table `accesstokens`
--

CREATE TABLE IF NOT EXISTS `accesstokens` (
  `token` binary(16) NOT NULL,
  `user_id` int(6) NOT NULL,
  `sudo` tinyint(1) NOT NULL DEFAULT '0',
  `one_time` tinyint(1) NOT NULL DEFAULT '0',
  `created` datetime NOT NULL,
  `expires` datetime DEFAULT NULL,
  `user_agent` varchar(255) NOT NULL,
  PRIMARY KEY (`token`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;

-- --------------------------------------------------------

--
-- Table structure for table `appcontent`
--

CREATE TABLE IF NOT EXISTS `appcontent` (
  `id` varchar(36) NOT NULL,
  `content` mediumtext CHARACTER SET utf8 COLLATE utf8_bin NOT NULL,
  `updated` datetime NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (`id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;

-- --------------------------------------------------------

--
-- Table structure for table `collectionattributes`
--

CREATE TABLE IF NOT EXISTS `collectionattributes` (
  `collection_id` int(6) NOT NULL,
  `attribute` varchar(128) NOT NULL,
  `value` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  PRIMARY KEY (`collection_id`,`attribute`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;

-- --------------------------------------------------------

--
-- Table structure for table `collectionkeys`
--

CREATE TABLE IF NOT EXISTS `collectionkeys` (
  `collection_id` int(6) NOT NULL,
  `public_key` blob NOT NULL,
  `private_key` blob NOT NULL,
  PRIMARY KEY (`collection_id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;

-- --------------------------------------------------------

--
-- Table structure for table `collectionpasswords`
--

CREATE TABLE IF NOT EXISTS `collectionpasswords` (
  `collection_id` int(6) NOT NULL,
  `password` char(60) NOT NULL,
  PRIMARY KEY (`collection_id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;

-- --------------------------------------------------------

--
-- Table structure for table `collectionredirects`
--

CREATE TABLE IF NOT EXISTS `collectionredirects` (
  `prev_alias` varchar(100) NOT NULL,
  `new_alias` varchar(100) NOT NULL,
  PRIMARY KEY (`prev_alias`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;

-- --------------------------------------------------------

--
-- Table structure for table `collections`
--

CREATE TABLE IF NOT EXISTS `collections` (
  `id` int(6) NOT NULL AUTO_INCREMENT,
  `alias` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin DEFAULT NULL,
  `title` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  `description` varchar(160) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  `style_sheet` text,
  `script` text CHARACTER SET utf8mb4 COLLATE utf8mb4_bin,
  `format` varchar(8) DEFAULT NULL,
  `privacy` tinyint(1) NOT NULL,
  `owner_id` int(6) NOT NULL,
  `view_count` int(6) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `alias` (`alias`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;

-- --------------------------------------------------------

--
-- Table structure for table `posts`
--

CREATE TABLE IF NOT EXISTS `posts` (
  `id` char(16) NOT NULL,
  `slug` varchar(100) DEFAULT NULL,
  `modify_token` char(32) DEFAULT NULL,
  `text_appearance` char(4) NOT NULL DEFAULT 'norm',
  `language` char(2) DEFAULT NULL,
  `rtl` tinyint(1) DEFAULT NULL,
  `privacy` tinyint(1) NOT NULL,
  `owner_id` int(6) DEFAULT NULL,
  `collection_id` int(6) DEFAULT NULL,
  `pinned_position` tinyint(1) UNSIGNED DEFAULT NULL,
  `created` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `updated` timestamp NOT NULL DEFAULT CURRENT_TIMESTAMP,
  `view_count` int(6) NOT NULL,
  `title` varchar(160) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  `content` text CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `id_slug` (`collection_id`,`slug`),
  UNIQUE KEY `owner_id` (`owner_id`,`id`),
  KEY `privacy_id` (`privacy`,`id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;

-- --------------------------------------------------------

--
-- Table structure for table `remotefollows`
--

CREATE TABLE IF NOT EXISTS `remotefollows` (
  `collection_id` int(11) NOT NULL,
  `remote_user_id` int(11) NOT NULL,
  `created` datetime NOT NULL,
  PRIMARY KEY (`collection_id`,`remote_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;

-- --------------------------------------------------------

--
-- Table structure for table `remoteuserkeys`
--

CREATE TABLE IF NOT EXISTS `remoteuserkeys` (
  `id` varchar(255) NOT NULL,
  `remote_user_id` int(11) NOT NULL,
  `public_key` blob NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `follower_id` (`remote_user_id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;

-- --------------------------------------------------------

--
-- Table structure for table `remoteusers`
--

CREATE TABLE IF NOT EXISTS `remoteusers` (
  `id` int(11) NOT NULL AUTO_INCREMENT,
  `actor_id` varchar(255) NOT NULL,
  `inbox` varchar(255) NOT NULL,
  `shared_inbox` varchar(255) NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `collection_id` (`actor_id`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;

-- --------------------------------------------------------

--
-- Table structure for table `userattributes`
--

CREATE TABLE IF NOT EXISTS `userattributes` (
  `user_id` int(6) NOT NULL,
  `attribute` varchar(64) NOT NULL,
  `value` varchar(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  PRIMARY KEY (`user_id`,`attribute`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;

-- --------------------------------------------------------

--
-- Table structure for table `users`
--

CREATE TABLE IF NOT EXISTS `users` (
  `id` int(6) NOT NULL AUTO_INCREMENT,
  `username` varchar(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_bin NOT NULL,
  `password` char(60) CHARACTER SET latin1 COLLATE latin1_bin NOT NULL,
  `email` varbinary(255) DEFAULT NULL,
  `created` datetime NOT NULL,
  PRIMARY KEY (`id`),
  UNIQUE KEY `username` (`username`)
) ENGINE=InnoDB DEFAULT CHARSET=latin1;
