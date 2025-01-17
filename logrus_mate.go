package logrus_mate

import (
	"errors"
	"io"
	"strings"
	"sync"

	"github.com/gogap/config"

	"github.com/45hur/logrus"
)

var (
	ErrLoggerNotExist = errors.New("logger not exist")
)

type LogrusMate struct {
	loggersConf sync.Map //map[string]*Config
	loggers     sync.Map //map[string]*logrus.Logger
}

func NewLogger(opts ...Option) (logger *logrus.Logger, err error) {
	l := logrus.New()
	if err = Hijack(l, opts...); err != nil {
		return
	}

	return l, nil
}

func Hijack(logger *logrus.Logger, opts ...Option) (err error) {

	logrusMateConf := Config{}
	for _, o := range opts {
		o(&logrusMateConf)
	}

	hijackConf := config.NewConfig(logrusMateConf.configOpts...)

	return hijackByConfig(logger, hijackConf)
}

func hijackByConfig(logger *logrus.Logger, conf config.Configuration) (err error) {
	if conf == nil {
		return
	}

	outConf := conf.GetConfig("out")
	formatterConf := conf.GetConfig("formatter")

	outName := "stdout"
	formatterName := "text"

	var outOptionsConf, formatterOptionsConf config.Configuration

	if outConf != nil {
		outName = outConf.GetString("name", "stdout")
		outOptionsConf = outConf.GetConfig("options")
	}

	if formatterConf != nil {
		formatterName = formatterConf.GetString("name", "text")
		formatterOptionsConf = formatterConf.GetConfig("options")
	}

	var out io.Writer
	if out, err = NewWriter(outName, outOptionsConf); err != nil {
		return
	}

	var formatter logrus.Formatter
	if formatter, err = NewFormatter(formatterName, formatterOptionsConf); err != nil {
		return
	}

	var hooks []logrus.Hook

	confHooks := conf.GetConfig("hooks")

	if confHooks != nil {
		hookNames := confHooks.Keys()

		for i := 0; i < len(hookNames); i++ {
			var hook logrus.Hook
			if hook, err = NewHook(hookNames[i], confHooks.GetConfig(hookNames[i])); err != nil {
				return
			}
			hooks = append(hooks, hook)
		}
	}

	level := conf.GetString("level")

	if len(level) == 0 {
		level = "info"
	}

	var lvl = logrus.DebugLevel
	if lvl, err = logrus.ParseLevel(level); err != nil {
		return
	}

	l := logrus.New()

	l.Level = lvl
	l.Out = out
	l.Formatter = formatter
	for i := 0; i < len(hooks); i++ {
		l.Hooks.Add(hooks[i])
	}

	*logger = *l

	return
}

func NewLogrusMate(opts ...Option) (logrusMate *LogrusMate, err error) {
	mate := &LogrusMate{
		loggersConf: sync.Map{},
		loggers:     sync.Map{},
	}

	logrusMateConf := Config{}
	for _, o := range opts {
		o(&logrusMateConf)
	}

	conf := config.NewConfig(logrusMateConf.configOpts...)

	if conf == nil {
		logrusMate = mate
		return
	}

	loggerNames := conf.Keys()

	for i := 0; i < len(loggerNames); i++ {
		mate.loggersConf.LoadOrStore(loggerNames[i], conf.GetConfig(loggerNames[i]))
	}

	logrusMate = mate

	return
}

func (p *LogrusMate) Hijack(logger *logrus.Logger, loggerName string, opts ...Option) (err error) {
	confV, exist := p.loggersConf.Load(loggerName)
	if !exist {
		err = ErrLoggerNotExist
		return
	}

	conf := confV.(config.Configuration)

	if len(opts) > 0 {

		newConf := Config{}
		for _, o := range opts {
			o(&newConf)
		}

		conf2 := config.NewConfig(newConf.configOpts...)

		err = hijackByConfig(
			logger,
			conf.WithFallback(conf2.Configuration),
		)

		return
	}

	err = hijackByConfig(logger, confV.(config.Configuration))

	return
}

func (p *LogrusMate) Logger(loggerName ...string) (logger *logrus.Logger) {
	name := "default"

	if len(loggerName) > 0 {
		name = strings.TrimSpace(loggerName[0])
		if len(name) == 0 {
			name = "default"
		}
	}

	lv, exist := p.loggers.Load(name)

	if exist {
		return lv.(*logrus.Logger)
	}

	confV, exist := p.loggersConf.Load(name)
	if !exist {
		return nil
	}

	l := logrus.New()

	if err := hijackByConfig(l, confV.(config.Configuration)); err != nil {
		return nil
	}

	p.loggers.LoadOrStore(name, l)

	return l
}

func (p *LogrusMate) LoggerNames() []string {
	var keys []string

	p.loggersConf.Range(func(key, value interface{}) bool {
		if kk, ok := key.(string); ok {
			keys = append(keys, kk)
		}
		return true
	})

	return keys
}
