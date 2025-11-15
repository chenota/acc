(in-package :acc)

(fiveam:def-suite token-sequence)

(def-fixture
    token-sequence-test-env ()
  (let* ((raw-seq-len 10)
         (raw-seq
          (loop
         for i from 0 below raw-seq-len
         collect (make-token :kind :test-token :value i :row 0 :col 0 :len 0))))
    (&body)))

(test test-initialization
  (with-fixture token-sequence-test-env ()
    (is (make-token-sequence raw-seq))
    (is (make-token-sequence (coerce raw-seq 'vector)))
    (is (make-token-sequence nil))
    (signals error (make-token-sequence '("cat" "dog")))
    (signals error (make-token-sequence 100))))

(test test-peek
  (with-fixture token-sequence-test-env ()
    (is (= 0 (token-value (peek (make-token-sequence raw-seq)))))
    (is (eq :ENDMARKER (token-kind (peek (make-token-sequence nil)))))))

(test test-advance
  (with-fixture token-sequence-test-env ()
    (is (= 0 (token-value (advance (make-token-sequence raw-seq)))))
    (is (= 1 (token-value (let ((s (make-token-sequence raw-seq))) (advance s) (advance s)))))
    (is (eq :ENDMARKER (token-kind (let ((s (make-token-sequence raw-seq))) (dotimes (i raw-seq-len) (advance s)) (advance s)))))))

(test test-capture-restore
  (with-fixture token-sequence-test-env ()
    (is (= 0 (capture (make-token-sequence raw-seq))))
    (is (= 1 (let ((s (make-token-sequence raw-seq))) (advance s) (capture s))))
    (is (= raw-seq-len (let ((s (make-token-sequence raw-seq))) (restore s raw-seq-len) (capture s))))
    (signals error (restore (make-token-sequence raw-seq) (1+ raw-seq-len)))))