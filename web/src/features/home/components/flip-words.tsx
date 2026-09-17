/*
Copyright (C) 2023-2026 QuantumNous

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as
published by the Free Software Foundation, either version 3 of the
License, or (at your option) any later version.

This program is distributed in the hope that it will be useful,
but WITHOUT ANY WARRANTY; without even the implied warranty of
MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
GNU Affero General Public License for more details.

You should have received a copy of the GNU Affero General Public License
along with this program. If not, see <https://www.gnu.org/licenses/>.

For commercial licensing, please contact support@quantumnous.com
*/
import { useCallback, useEffect, useState } from 'react'
import { AnimatePresence, motion } from 'motion/react'
import { cn } from '@/lib/utils'

interface FlipWordsProps {
  words: string[]
  duration?: number
  className?: string
}

/**
 * Rotating headline words: each word staggers in letter by letter with a
 * blur, then exits with a scale-and-blur flourish.
 */
export function FlipWords({
  words,
  duration = 3000,
  className,
}: FlipWordsProps) {
  const [currentWord, setCurrentWord] = useState(words[0])
  const [isAnimating, setIsAnimating] = useState(false)

  const startAnimation = useCallback(() => {
    setCurrentWord((word) => words[words.indexOf(word) + 1] ?? words[0])
    setIsAnimating(true)
  }, [words])

  useEffect(() => {
    if (isAnimating) {
      return
    }
    const timer = setTimeout(startAnimation, duration)
    return () => clearTimeout(timer)
  }, [isAnimating, duration, startAnimation])

  return (
    <span
      className={cn(
        'relative inline-block min-h-[1.15em] text-left',
        className
      )}
    >
      <AnimatePresence mode='wait' onExitComplete={() => setIsAnimating(false)}>
        <motion.span
          key={currentWord}
          initial={{ opacity: 0, y: 10 }}
          animate={{ opacity: 1, y: 0 }}
          transition={{ type: 'spring', stiffness: 100, damping: 10 }}
          exit={{
            opacity: 0,
            y: -10,
            filter: 'blur(4px)',
          }}
          className='inline-block'
        >
          {currentWord.split(' ').map((word, wordIndex) => (
            <motion.span
              // oxlint-disable-next-line react/no-array-index-key
              key={`${word}-${wordIndex}`}
              initial={{ opacity: 0, y: 10, filter: 'blur(8px)' }}
              animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
              transition={{ delay: wordIndex * 0.3, duration: 0.3 }}
              className='inline-block whitespace-nowrap'
            >
              {[...word].map((letter, letterIndex) => (
                <motion.span
                  // oxlint-disable-next-line react/no-array-index-key
                  key={`${letter}-${letterIndex}`}
                  initial={{ opacity: 0, y: 10, filter: 'blur(8px)' }}
                  animate={{ opacity: 1, y: 0, filter: 'blur(0px)' }}
                  transition={{
                    delay: wordIndex * 0.3 + letterIndex * 0.05,
                    duration: 0.2,
                  }}
                  className='inline-block'
                >
                  {letter}
                </motion.span>
              ))}
              {wordIndex < currentWord.split(' ').length - 1 && (
                <span className='inline-block'>&nbsp;</span>
              )}
            </motion.span>
          ))}
        </motion.span>
      </AnimatePresence>
    </span>
  )
}
