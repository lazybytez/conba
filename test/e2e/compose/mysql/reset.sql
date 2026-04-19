DROP TABLE IF EXISTS widgets;
CREATE TABLE widgets (
  id INT PRIMARY KEY AUTO_INCREMENT,
  name VARCHAR(64) NOT NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
INSERT INTO widgets (name) VALUES ('alpha'),('beta'),('gamma'),('delta'),('epsilon');
