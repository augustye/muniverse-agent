package main

import (
	"time"
	"log"

	"github.com/unixpickle/essentials"
	"github.com/unixpickle/muniverse"
)

// An Env is an anyrl.Env which wraps a muniverse.Env.
type Env struct {
	RawEnv      muniverse.Env
	Actor       Actor
	Observer    Observer
	RewardScale float64

	FrameTime time.Duration
	MaxSteps  int

	timestep int
	joiner   *ObsJoiner
}

// NewEnv creates an environment according to the flags
// and specification.
//
// It is the caller's responsibility to close RawEnv once
// it is done using the environment.
func NewEnv(flags *TrainingFlags, spec *EnvSpec) *Env {
	opts := &muniverse.Options{}
	if flags.ImageName != "" {
		opts.CustomImage = flags.ImageName
	}
	if flags.GamesDir != "" {
		opts.GamesDir = flags.GamesDir
	}
	if flags.Compression >= 0 {
		if flags.Compression > 100 {
			essentials.Die("invalid compression level:", flags.Compression)
		}
		opts.Compression = true
		opts.CompressionQuality = flags.Compression
	}
	env, err := muniverse.NewEnvOptions(spec.EnvSpec, opts)
	if err != nil {
		essentials.Die(err)
	}
	if spec.Wrap != nil {
		env = spec.Wrap(env)
	}
	if flags.RecordDir != "" {
		env = muniverse.RecordEnv(env, flags.RecordDir)
	}
	return &Env{
		RawEnv:      env,
		Actor:       spec.MakeActor(),
		Observer:    spec.Observer,
		RewardScale: spec.RewardScale,
		FrameTime:   spec.FrameTime,
		MaxSteps:    flags.MaxSteps,
		joiner:      &ObsJoiner{HistorySize: spec.HistorySize},
	}
}

// Reset resets the environment.
func (e *Env) Reset() (obs []float64, err error) {
	log.Println("[Reset] start")
	defer essentials.AddCtxTo("reset", &err)

	e.Actor.Reset()
	e.timestep = 0

	err = e.RawEnv.Reset()
	if err != nil {
		log.Println("[Reset] RawEnv.Reset() error:", err)
		return
	}

	rawObs, err := e.RawEnv.Observe()
	if err != nil {
		log.Println("[Reset] RawEnv.Observe() error:", err)
		return
	}
	obsVec, err := e.Observer.ObsVec(rawObs)
	if err != nil {
		log.Println("[Reset] Observer.ObsVec() error:", err)
		return
	}
	e.joiner.Reset(obsVec)
	obs = e.joiner.Step(obsVec)

	log.Println("[Reset] done")
	return
}

// Step takes a step in the environment.
func (e *Env) Step(action []float64) (obs []float64, reward float64,
	done bool, err error) {
	events := e.Actor.Events(action)
	reward, done, err = e.RawEnv.Step(e.FrameTime, events...)
	if err != nil {
		return
	}
	if e.RewardScale != 0 {
		reward *= e.RewardScale
	}

	rawObs, err := e.RawEnv.Observe()
	if err != nil {
		log.Println("[Step] RawEnv.Observe() error:", err)
		return
	}
	obsVec, err := e.Observer.ObsVec(rawObs)
	if err != nil {
		log.Println("[Step] Observer.ObsVec() error:", err)
		return
	}
	obs = e.joiner.Step(obsVec)

	e.timestep++
	if e.timestep >= e.MaxSteps {
		done = true
	}

	return
}
