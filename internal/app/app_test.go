package app

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	configmocks "github.com/joshuarp/withdraw-api/internal/mock/shared/config"
)

type AppHelpersSuite struct {
	suite.Suite

	cfg *configmocks.ConfigProvider
}

func (s *AppHelpersSuite) SetupTest() {
	s.cfg = configmocks.NewConfigProvider(s.T())
}

func (s *AppHelpersSuite) TestIsSingleBinaryBin_TableDriven() {
	tests := []struct {
		name   string
		bin    string
		expect bool
	}{
		{name: "empty is single binary", bin: "", expect: true},
		{name: "all is single binary", bin: "all", expect: true},
		{name: "mixed case all is single binary", bin: " All ", expect: true},
		{name: "inquiry is module binary", bin: "inquiry", expect: false},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			assert.Equal(s.T(), tc.expect, isSingleBinaryBin(tc.bin))
		})
	}
}

func (s *AppHelpersSuite) TestModuleDBString_TableDriven() {
	tests := []struct {
		name            string
		useModuleConfig bool
		setupMock       func()
		expect          string
	}{
		{
			name:            "prefer module yaml key",
			useModuleConfig: true,
			setupMock: func() {
				s.cfg.EXPECT().IsSet("database.wallet.host").Return(true)
				s.cfg.EXPECT().GetString("database.wallet.host").Return("wallet-host")
			},
			expect: "wallet-host",
		},
		{
			name:            "fallback to module env key",
			useModuleConfig: true,
			setupMock: func() {
				s.cfg.EXPECT().IsSet("database.wallet.host").Return(false)
				s.cfg.EXPECT().IsSet("DATABASE_WALLET_HOST").Return(true)
				s.cfg.EXPECT().GetString("DATABASE_WALLET_HOST").Return("wallet-env-host")
			},
			expect: "wallet-env-host",
		},
		{
			name:            "fallback to global yaml key",
			useModuleConfig: false,
			setupMock: func() {
				s.cfg.EXPECT().IsSet("database.host").Return(true)
				s.cfg.EXPECT().GetString("database.host").Return("global-host")
			},
			expect: "global-host",
		},
		{
			name:            "fallback to global env key",
			useModuleConfig: false,
			setupMock: func() {
				s.cfg.EXPECT().IsSet("database.host").Return(false)
				s.cfg.EXPECT().GetString("DATABASE_HOST").Return("global-env-host")
			},
			expect: "global-env-host",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			if tc.setupMock != nil {
				tc.setupMock()
			}

			value := moduleDBString(s.cfg, "wallet", "host", tc.useModuleConfig)
			assert.Equal(s.T(), tc.expect, value)
		})
	}
}

func (s *AppHelpersSuite) TestModuleDBInt_TableDriven() {
	tests := []struct {
		name            string
		useModuleConfig bool
		setupMock       func()
		expect          int
	}{
		{
			name:            "prefer module yaml int",
			useModuleConfig: true,
			setupMock: func() {
				s.cfg.EXPECT().IsSet("database.wallet.port").Return(true)
				s.cfg.EXPECT().GetInt("database.wallet.port").Return(5433)
			},
			expect: 5433,
		},
		{
			name:            "fallback to global env int",
			useModuleConfig: false,
			setupMock: func() {
				s.cfg.EXPECT().IsSet("database.port").Return(false)
				s.cfg.EXPECT().GetInt("DATABASE_PORT").Return(5432)
			},
			expect: 5432,
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			if tc.setupMock != nil {
				tc.setupMock()
			}

			value := moduleDBInt(s.cfg, "wallet", "port", tc.useModuleConfig)
			assert.Equal(s.T(), tc.expect, value)
		})
	}
}

func (s *AppHelpersSuite) TestProvideFiberApp_TableDriven() {
	tests := []struct {
		name       string
		readValue  time.Duration
		writeValue time.Duration
	}{
		{name: "defaults when config missing", readValue: 0, writeValue: 0},
		{name: "uses configured timeout", readValue: 10 * time.Second, writeValue: 12 * time.Second},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.cfg.EXPECT().GetDuration("server.read_timeout").Return(tc.readValue)
			s.cfg.EXPECT().GetDuration("server.write_timeout").Return(tc.writeValue)

			fiberApp := provideFiberApp(s.cfg)
			assert.NotNil(s.T(), fiberApp)
		})
	}
}

func (s *AppHelpersSuite) TestProvideJWTTokenManager_TableDriven() {
	tests := []struct {
		name      string
		setupMock func()
		assertion func(error)
	}{
		{
			name: "uses security jwt secret and ttl",
			setupMock: func() {
				s.cfg.EXPECT().GetString("security.jwt.secret").Return("12345678901234567890123456789012")
				s.cfg.EXPECT().GetDuration("security.jwt.ttl").Return(15 * time.Minute)
				s.cfg.EXPECT().GetString("security.jwt.issuer").Return("withdraw-api")
			},
			assertion: func(err error) {
				assert.NoError(s.T(), err)
			},
		},
		{
			name: "fallback to legacy jwt secret and default ttl",
			setupMock: func() {
				s.cfg.EXPECT().GetString("security.jwt.secret").Return("")
				s.cfg.EXPECT().GetString("jwt.secret").Return("legacy")
				s.cfg.EXPECT().GetDuration("security.jwt.ttl").Return(time.Duration(0))
				s.cfg.EXPECT().GetString("security.jwt.issuer").Return("issuer")
			},
			assertion: func(err error) {
				assert.NoError(s.T(), err)
			},
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			tc.setupMock()

			manager, err := provideJWTTokenManager(s.cfg)
			assert.NotNil(s.T(), manager)
			tc.assertion(err)
		})
	}
}

func (s *AppHelpersSuite) TestProvideRedisClient_TableDriven() {
	tests := []struct {
		name      string
		host      string
		port      int
		password  string
		db        int
		expAddr   string
		expDB     int
		expPasswd string
	}{
		{
			name:      "uses configured redis settings",
			host:      "redis.internal",
			port:      6380,
			password:  "topsecret",
			db:        2,
			expAddr:   "redis.internal:6380",
			expDB:     2,
			expPasswd: "topsecret",
		},
		{
			name:      "uses default host and port when not configured",
			host:      "",
			port:      0,
			password:  "",
			db:        0,
			expAddr:   "localhost:6379",
			expDB:     0,
			expPasswd: "",
		},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			s.SetupTest()
			s.cfg.EXPECT().GetString("redis.host").Return(tc.host)
			s.cfg.EXPECT().GetInt("redis.port").Return(tc.port)
			s.cfg.EXPECT().GetString("redis.password").Return(tc.password)
			s.cfg.EXPECT().GetInt("redis.db").Return(tc.db)

			client := provideRedisClient(s.cfg)
			if client != nil {
				defer client.Close()
			}

			require := assert.New(s.T())
			require.NotNil(client)

			opts := client.Options()
			assert.Equal(s.T(), tc.expAddr, opts.Addr)
			assert.Equal(s.T(), tc.expDB, opts.DB)
			assert.Equal(s.T(), tc.expPasswd, opts.Password)
		})
	}
}

func (s *AppHelpersSuite) TestParseRateLimitAlgorithm_TableDriven() {
	tests := []struct {
		name   string
		input  string
		expect string
	}{
		{name: "default token bucket", input: "", expect: "token_bucket"},
		{name: "sliding window", input: "sliding_window", expect: "sliding_window"},
		{name: "fixed window", input: "fixed_window", expect: "fixed_window"},
		{name: "unknown falls back", input: "random", expect: "token_bucket"},
	}

	for _, tc := range tests {
		s.Run(tc.name, func() {
			assert.Equal(s.T(), tc.expect, string(parseRateLimitAlgorithm(tc.input)))
		})
	}
}

func TestAppHelpersSuite(t *testing.T) {
	suite.Run(t, new(AppHelpersSuite))
}
