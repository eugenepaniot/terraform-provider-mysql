package mysql

import (
	"fmt"
	"log"
	"math/rand"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
	"github.com/hashicorp/go-version"
	"github.com/hashicorp/terraform-plugin-sdk/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/terraform"
)

func TestAccGrant(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfig_basic(dbName),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilegeExists("mysql_grant.test", "SELECT"),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "*"),
					resource.TestCheckResourceAttr("mysql_grant.test", "tls_option", "NONE"),
				),
			},
			{
				Config: testAccGrantConfig_ssl(dbName),
				Check: resource.ComposeTestCheckFunc(
					testAccPrivilegeExists("mysql_grant.test", "SELECT"),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "tls_option", "SSL"),
				),
			},
		},
	})
}

func TestAccGrantComplex(t *testing.T) {
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck:     func() { testAccPreCheck(t) },
		Providers:    testAccProviders,
		CheckDestroy: testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				// Create table first
				Config: testAccGrantConfig_nogrant(dbName),
				Check: resource.ComposeTestCheckFunc(
					prepareTable(dbName),
				),
			},
			{
				Config: testAccGrantConfig_with_privs(dbName, `"SELECT (c1, c2)"`),
				Check: resource.ComposeTestCheckFunc(
					// TODO: grant parsing?
					//testAccPrivilegeExists("mysql_grant.test", "SELECT (c1, c2)"),
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "tbl"),
					resource.TestCheckResourceAttr("mysql_grant.test", "tls_option", "NONE"),
				),
			},
			{
				Config: testAccGrantConfig_with_privs(dbName, `"DROP", "SELECT (c1)", "INSERT(c3, c4)", "REFERENCES(c5)"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "tbl"),
					resource.TestCheckResourceAttr("mysql_grant.test", "tls_option", "NONE"),
				),
			},
			{
				Config: testAccGrantConfig_with_privs(dbName, `"DROP", "SELECT (c1)", "INSERT(c4, c3, c2)"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "tbl"),
					resource.TestCheckResourceAttr("mysql_grant.test", "tls_option", "NONE"),
				),
			},
			{
				Config: testAccGrantConfig_with_privs(dbName, `"ALL PRIVILEGES"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "tbl"),
					resource.TestCheckResourceAttr("mysql_grant.test", "tls_option", "NONE"),
				),
			},
			{
				Config: testAccGrantConfig_with_privs(dbName, `"ALL"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "tbl"),
					resource.TestCheckResourceAttr("mysql_grant.test", "tls_option", "NONE"),
				),
			},
			{
				Config: testAccGrantConfig_with_privs(dbName, `"DROP", "SELECT (c1, c2)", "INSERT(c5)", "REFERENCES(c1)"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "database", dbName),
					resource.TestCheckResourceAttr("mysql_grant.test", "table", "tbl"),
					resource.TestCheckResourceAttr("mysql_grant.test", "tls_option", "NONE"),
				),
			},
		},
	})
}

func TestAccGrant_role(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	roleName := fmt.Sprintf("TFRole%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			db, err := connectToMySQL(testAccProvider.Meta().(*MySQLConfiguration))
			if err != nil {
				return
			}

			requiredVersion, _ := version.NewVersion("8.0.0")
			currentVersion, err := serverVersion(db)
			if err != nil {
				return
			}

			if currentVersion.LessThan(requiredVersion) {
				t.Skip("Roles require MySQL 8+")
			}
		},
		Providers:    testAccProviders,
		CheckDestroy: testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfig_role(dbName, roleName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("mysql_grant.test", "role", roleName),
				),
			},
		},
	})
}

func TestAccGrant_roleToUser(t *testing.T) {
	rand.Seed(time.Now().UnixNano())
	dbName := fmt.Sprintf("tf-test-%d", rand.Intn(100))
	roleName := fmt.Sprintf("TFRole%d", rand.Intn(100))
	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			db, err := connectToMySQL(testAccProvider.Meta().(*MySQLConfiguration))
			if err != nil {
				return
			}

			requiredVersion, _ := version.NewVersion("8.0.0")
			currentVersion, err := serverVersion(db)
			if err != nil {
				return
			}

			if currentVersion.LessThan(requiredVersion) {
				t.Skip("Roles require MySQL 8+")
			}
		},
		Providers:    testAccProviders,
		CheckDestroy: testAccGrantCheckDestroy,
		Steps: []resource.TestStep{
			{
				Config: testAccGrantConfig_roleToUser(dbName, roleName),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("mysql_grant.test", "user", fmt.Sprintf("jdoe-%s", dbName)),
					resource.TestCheckResourceAttr("mysql_grant.test", "host", "example.com"),
					resource.TestCheckResourceAttr("mysql_grant.test", "roles.#", "1"),
				),
			},
		},
	})
}

func prepareTable(dbname string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		db, err := connectToMySQL(testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}
		if _, err := db.Exec(fmt.Sprintf("CREATE TABLE `%s`.`tbl`(c1 INT, c2 INT, c3 INT,c4 INT,c5 INT);", dbname)); err != nil {
			return fmt.Errorf("error reading grant: %s", err)
		}
		return nil
	}
}

func testAccPrivilegeExists(rn string, privilege string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[rn]
		if !ok {
			return fmt.Errorf("resource not found: %s", rn)
		}

		if rs.Primary.ID == "" {
			return fmt.Errorf("grant id not set")
		}

		db, err := connectToMySQL(testAccProvider.Meta().(*MySQLConfiguration))
		if err != nil {
			return err
		}

		id := strings.Split(rs.Primary.ID, ":")

		var userOrRole string
		if strings.Contains(id[0], "@") {
			parts := strings.Split(id[0], "@")
			userOrRole = fmt.Sprintf("'%s'@'%s'", parts[0], parts[1])
		} else {
			userOrRole = fmt.Sprintf("'%s'", id[0])
		}

		stmtSQL := fmt.Sprintf("SHOW GRANTS FOR %s", userOrRole)
		log.Printf("[DEBUG] SQL: %s", stmtSQL)
		rows, err := db.Query(stmtSQL)
		if err != nil {
			return fmt.Errorf("error reading grant: %s", err)
		}
		defer rows.Close()

		privilegeFound := false
		for rows.Next() {
			var grants string
			err = rows.Scan(&grants)
			if err != nil {
				return fmt.Errorf("failed to read grant for %s: %s", userOrRole, err)
			}
			log.Printf("[DEBUG] GRANTS = %s", grants)
			privIndex := strings.Index(grants, privilege)
			if privIndex != -1 {
				privilegeFound = true
			}
		}

		if !privilegeFound {
			return fmt.Errorf("grant %s no found for %s", privilege, userOrRole)
		}

		return nil
	}
}

func testAccGrantCheckDestroy(s *terraform.State) error {
	db, err := connectToMySQL(testAccProvider.Meta().(*MySQLConfiguration))
	if err != nil {
		return err
	}

	for _, rs := range s.RootModule().Resources {
		if rs.Type != "mysql_grant" {
			continue
		}

		id := strings.Split(rs.Primary.ID, ":")

		var userOrRole string
		if strings.Contains(id[0], "@") {
			parts := strings.Split(id[0], "@")
			userOrRole = fmt.Sprintf("'%s'@'%s'", parts[0], parts[1])
		} else {
			userOrRole = fmt.Sprintf("'%s'", id[0])
		}

		stmtSQL := fmt.Sprintf("SHOW GRANTS FOR %s", userOrRole)
		log.Printf("[DEBUG] SQL: %s", stmtSQL)
		rows, err := db.Query(stmtSQL)
		if err != nil {
			if mysqlErr, ok := err.(*mysql.MySQLError); ok {
				if mysqlErr.Number == nonexistingGrantErrCode {
					return nil
				}
			}

			return fmt.Errorf("error reading grant: %s", err)
		}
		defer rows.Close()

		if rows.Next() {
			return fmt.Errorf("grant still exists for: %s", userOrRole)
		}
	}
	return nil
}

func testAccGrantConfig_nogrant(dbName string) string {
	return fmt.Sprintf(`
resource "mysql_database" "test" {
  name = "%s"
}

resource "mysql_user" "test" {
  user     = "jdoe-%s"
  host     = "example.com"
}
`, dbName, dbName)
}

func testAccGrantConfig_with_privs(dbName, privs string) string {
	return fmt.Sprintf(`
resource "mysql_database" "test" {
  name = "%s"
}

resource "mysql_user" "test" {
  user     = "jdoe-%s"
  host     = "example.com"
}

resource "mysql_grant" "test" {
  user       = "${mysql_user.test.user}"
  host       = "${mysql_user.test.host}"
  table      = "tbl"
  database   = "${mysql_database.test.name}"
  privileges = [%s]
}
`, dbName, dbName, privs)
}

func testAccGrantConfig_basic(dbName string) string {
	return fmt.Sprintf(`
resource "mysql_database" "test" {
  name = "%s"
}

resource "mysql_user" "test" {
  user     = "jdoe-%s"
  host     = "example.com"
}

resource "mysql_grant" "test" {
  user       = "${mysql_user.test.user}"
  host       = "${mysql_user.test.host}"
  database   = "${mysql_database.test.name}"
  privileges = ["UPDATE", "SELECT"]
}
`, dbName, dbName)
}

func testAccGrantConfig_ssl(dbName string) string {
	return fmt.Sprintf(`
resource "mysql_database" "test" {
  name = "%s"
}

resource "mysql_user" "test" {
  user     = "jdoe-%s"
  host     = "example.com"
}

resource "mysql_grant" "test" {
  user       = "${mysql_user.test.user}"
  host       = "${mysql_user.test.host}"
  database   = "${mysql_database.test.name}"
  privileges = ["UPDATE", "SELECT"]
  tls_option = "SSL"
}
`, dbName, dbName)
}

func testAccGrantConfig_role(dbName string, roleName string) string {
	return fmt.Sprintf(`
resource "mysql_database" "test" {
  name = "%s"
}

resource "mysql_role" "test" {
  name = "%s"
}

resource "mysql_grant" "test" {
  role       = "${mysql_role.test.name}"
  database   = "${mysql_database.test.name}"
  privileges = ["SELECT", "UPDATE"]
}
`, dbName, roleName)
}

func testAccGrantConfig_roleToUser(dbName string, roleName string) string {
	return fmt.Sprintf(`
resource "mysql_database" "test" {
  name = "%s"
}

resource "mysql_user" "jdoe" {
  user     = "jdoe-%s"
  host     = "example.com"
}

resource "mysql_role" "test" {
  name = "%s"
}

resource "mysql_grant" "test" {
  user     = "${mysql_user.jdoe.user}"
  host     = "${mysql_user.jdoe.host}"
  database = "${mysql_database.test.name}"
  roles    = ["${mysql_role.test.name}"]
}
`, dbName, dbName, roleName)
}
