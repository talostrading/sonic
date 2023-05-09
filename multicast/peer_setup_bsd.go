//go:build darwin || netbsd || freebsd || openbsd || dragonfly

package multicast

// SetupPeer on BSD. This shouldn't be here but BSD, well at least macOS, has
// some messed up routing going on.
//
// Normally SetLoop is true by default. This means that you can have a writer
// and a reader on the same host and the reader can read what the writer writes
// to a group, if the reader joined that group. Setting it to false will make it
// such that the reader cannot read what the writer writes if they're both on
// the same host. You can see how having it set to true this is useful for
// testing.
//
// Now things are a bit different on macOS, and if you know why, please ping me.
// If SetLoop is true, which is the default, the reader gets each packet the
// writer writes twice. Setting it to false makes it such that the reader only
// receives each packet once. This implies there some hidden routing rule
// somewhere but I'm not sure how to read those yet.
//
//
func SetupPeer(p *UDPPeer) error {
    if err := p.SetLoop(false); err != nil {
        return err
    }
    return nil
}
