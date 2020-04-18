package mediaPlayer

import (
	"errors"
	"io"
	"math"
	"os"
	"os/exec"
	"strconv"
	"time"
)

/////EXAMPLE/////
/*func main() {

	m := NewMedia("test.mp4")
	m.SetPlayer(OMXPlayer)
	m.SetPosition(40)
	m.SetVolume(90)
	if err := m.Play(); err != nil {
		log.Fatalln(err)
	}
	time.Sleep(time.Second * 3)
	m.SetVolume(0)
	// time.Sleep(time.Second * 3)
	// if err := m.Pause(); err != nil {
	// 	 log.Fatalln(err)
	// }
	// time.Sleep(time.Second * 3)
	// if err := m.Play(); err != nil {
	// 	 log.Fatalln(err)
	// }
	// time.Sleep(time.Second * 3)
	// if err := m.Stop(); err != nil {
	// 	 log.Fatalln(err)
	// }
}*/

var (
	defaultPlayer           int
	defaultPlayerOSDLevel   int
	defaultPlayerFullScreen bool
)

type Media struct {
	fileName string
	cmd      *exec.Cmd
	writer   io.WriteCloser
	reader   io.ReadCloser
	IsOpen   bool
	IsPlay   bool
	IsMute   bool
	oldVol   int
	vol      int
	pos      int
	rotate   int
	repeat   bool

	player     int
	osdLevel   int
	fullScreen bool

	extraCmd []string
}

func SetDefaultPlayer(player int) {
	if player > 2 {
		return
	}
	defaultPlayer = player
}

func SetDefaultPlayerOSD(osdLevel int) {
	defaultPlayerOSDLevel = osdLevel
}

func SetDefaultPlayerFullScreen(fs bool) {
	defaultPlayerFullScreen = fs
}

func NewMedia(fileName string) *Media {
	return &Media{fileName: fileName, vol: 100, player: defaultPlayer, osdLevel: defaultPlayerOSDLevel, fullScreen: defaultPlayerFullScreen}
}

func (m *Media) SetFileName(fileName string) {
	m.fileName = fileName
}

func (m *Media) SetVolume(vol int) {
	if vol > 100 {
		vol = 100
	}
	if m.IsOpen {
		go m.setTargetVolume(vol)
	} else {
		m.vol = vol
	}
}

func (m *Media) SetPosition(second int) {
	m.pos = second
}

func (m *Media) SetRotate(orient int) {
	m.rotate = orient
}

func (m *Media) SetRepeat(repeat bool) {
	m.repeat = repeat
}

func (m *Media) SetPlayer(player int) {
	if player > 1 {
		return
	}
	m.player = player
}

func (m *Media) SetFullScreen(fs bool) {
	m.fullScreen = fs
}

func (m *Media) SetOSDLevel(osdLevel int) {
	m.osdLevel = osdLevel
}

func (m *Media) SetExtraCmd(cmd []string) {
	m.extraCmd = cmd
}

func (m *Media) PlayerStdout() io.ReadCloser {
	return m.reader
}

func (m *Media) Play() error {
	return m.play()
}

func (m *Media) Pause() error {
	return m.pause()
}

func (m *Media) Stop() error {
	return m.stop()
}

func (m *Media) Mute() {
	go m.mute()
}

func (m *Media) Unmute() {
	go m.unmute()
}

func (m *Media) open() error {
	args := []string{}
	switch m.player {

	case MPlayer:
		args = append(args, "mplayer")
		args = append(args, "-volume")
		args = append(args, strconv.Itoa(m.vol))
		if m.rotate > 0 {
			if m.rotate <= 90 {
				args = append(args, "-vf")
				args = append(args, "rotate=1")
			} else if m.rotate <= 180 {
				args = append(args, "-vf")
				args = append(args, "mirror")
				args = append(args, "-flip")
			} else if m.rotate <= 270 {
				args = append(args, "-vf")
				args = append(args, "rotate=2")
			}
		}
		if m.repeat {
			args = append(args, "-loop")
			args = append(args, "0")
		}
		if m.pos > 0 {
			args = append(args, "-ss")
			args = append(args, strconv.Itoa(m.pos))
		}
		args = append(args, "-osdlevel")
		args = append(args, strconv.Itoa(m.osdLevel))
		if m.fullScreen {
			args = append(args, "-fs")
		}

	case OMXPlayer:
		args = append(args, "omxplayer")
		nVol := 2000 * math.Log10(float64(m.vol)/100)
		args = append(args, "--vol")
		args = append(args, strconv.Itoa(int(nVol)))
		if m.rotate > 0 {
			args = append(args, "--orientation")
			args = append(args, strconv.Itoa(m.rotate))
		}
		if m.repeat {
			args = append(args, "--loop")
		}
		if m.pos > 0 {
			args = append(args, "-l")
			args = append(args, strconv.Itoa(m.pos))
		}
	}
	args = append(args, m.extraCmd...)
	args = append(args, m.fileName)

	cmd := exec.Command(args[0], args[1:]...)
	cmdWriter, err := cmd.StdinPipe()
	cmdReader, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}

	err = cmd.Start()
	if err != nil {
		return err
	}

	m.IsOpen = true
	m.IsPlay = true

	m.writer = cmdWriter
	m.reader = cmdReader
	m.cmd = cmd
	go func() {
		cmd.Wait()
		m.IsOpen = false
		m.IsPlay = false
	}()

	return nil
}

func (m *Media) play() error {
	if m.fileName == "" {
		return errors.New("Filename is empty.")
	}
	_, err := os.Stat(m.fileName)
	if os.IsNotExist(err) {
		return err
	}
	if !m.IsPlay {
		if !m.IsOpen {
			return m.open()
		} else {
			if _, err := m.writer.Write([]byte{KeySpace}); err != nil {
				return err
			}
			m.IsPlay = true
		}
	}
	return nil
}

func (m *Media) pause() error {
	if m.IsOpen && m.IsPlay {
		if _, err := m.writer.Write([]byte{KeySpace}); err != nil {
			return err
		}
		m.IsPlay = false
	}
	return nil
}

func (m *Media) stop() error {
	if m.IsOpen {
		if _, err := m.writer.Write([]byte{KeyQ}); err != nil {
			return err
		}
		m.cmd.Process.Kill()
		m.IsOpen = false
		m.IsPlay = false
	}
	return nil
}

func (m *Media) setTargetVolume(targetVol int) error {
	if !m.IsOpen || m.vol == targetVol {
		return nil
	}
	var inc bool

	if targetVol > m.vol {
		inc = true
	}

	switch m.player {
	case MPlayer:
		for !inc && m.vol-targetVol > 0 {
			if _, err := m.writer.Write([]byte{Key9}); err != nil {
				return err
			}
			m.vol -= 3
			time.Sleep(time.Millisecond * 20)
		}
		for inc && targetVol-m.vol > 0 {
			if _, err := m.writer.Write([]byte{Key0}); err != nil {
				return err
			}
			m.vol += 3
			time.Sleep(time.Millisecond * 20)
		}
	case OMXPlayer:
		for !inc && m.vol-targetVol > 0 {
			if _, err := m.writer.Write([]byte{KeyMinus}); err != nil {
				return err
			}
			m.vol -= 4
			time.Sleep(time.Millisecond * 20)
		}
		for inc && targetVol-m.vol > 0 {
			if _, err := m.writer.Write([]byte{KeyPlus}); err != nil {
				return err
			}
			m.vol += 4
			time.Sleep(time.Millisecond * 20)
		}
	}

	return nil
}

func (m *Media) mute() error {
	if !m.IsOpen && m.IsMute {
		return nil
	}
	switch m.player {
	case MPlayer:
		if _, err := m.writer.Write([]byte{KeyM}); err != nil {
			return err
		}
		m.IsMute = true
	case OMXPlayer:
		m.oldVol = m.vol
		if err := m.setTargetVolume(0); err != nil {
			return err
		}
		m.IsMute = true
	}

	return nil
}

func (m *Media) unmute() error {
	if !m.IsOpen && !m.IsMute {
		return nil
	}
	switch m.player {
	case MPlayer:
		if _, err := m.writer.Write([]byte{KeyM}); err != nil {
			return err
		}
		m.IsMute = false
	case OMXPlayer:
		if err := m.setTargetVolume(m.oldVol); err != nil {
			return err
		}
		m.IsMute = false
	}

	return nil
}
