import { CountdownCircleTimer } from 'react-countdown-circle-timer'

export const Clock = (size) => {
  return(
    <div class="countdown">
      <CountdownCircleTimer
        size="300"
        isPlaying
        duration={100}

        colors={[
          ['#004777', 0.33],
          ['#F7B801', 0.33],
          ['#A30000', 0.33],]}
        >
       {({ remainingTime }) => remainingTime}
     
      </CountdownCircleTimer>

  
    </div>
    
  );
}
/*
const children = ({ remainingTime }) => {
    const minutes = Math.floor(remainingTime / 60)
    const seconds = remainingTime % 60
  
    return `${minutes}:${seconds}`
    }
*/