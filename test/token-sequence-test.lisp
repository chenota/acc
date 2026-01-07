(in-package :acc)

(fiveam:def-suite token-sequence)

(fiveam:def-fixture
    token-sequence-test-env ()
  (let* ((raw-seq-len 10)
         (raw-seq
          (loop
         for i from 0 below raw-seq-len
         collect (make-token :kind :test-token :value i :row 0 :col 0 :len 0))))
    (&body)))

(fiveam:test test-initialization
  (fiveam:with-fixture token-sequence-test-env ()
    (fiveam:is (make-token-sequence raw-seq))
    (fiveam:is (make-token-sequence (coerce raw-seq 'vector)))
    (fiveam:is (make-token-sequence nil))
    (fiveam:signals error (make-token-sequence '("cat" "dog")))
    (fiveam:signals error (make-token-sequence 100))))

(fiveam:test test-peek
  (fiveam:with-fixture token-sequence-test-env ()
    (fiveam:is (= 0 (token-value (peek (make-token-sequence raw-seq)))))
    (fiveam:is (eq :ENDMARKER (token-kind (peek (make-token-sequence nil)))))))

(fiveam:test test-advance
  (fiveam:with-fixture token-sequence-test-env ()
    (fiveam:is (= 0 (token-value (advance (make-token-sequence raw-seq)))))
    (fiveam:is (= 1 (token-value (let ((s (make-token-sequence raw-seq))) (advance s) (advance s)))))
    (fiveam:is (eq :ENDMARKER (token-kind (let ((s (make-token-sequence raw-seq))) (dotimes (i raw-seq-len) (advance s)) (advance s)))))))

(fiveam:test test-capture-restore
  (fiveam:with-fixture token-sequence-test-env ()
    (fiveam:is (= 0 (capture (make-token-sequence raw-seq))))
    (fiveam:is (= 1 (let ((s (make-token-sequence raw-seq))) (advance s) (capture s))))
    (fiveam:is (= raw-seq-len (let ((s (make-token-sequence raw-seq))) (restore s raw-seq-len) (capture s))))
    (fiveam:signals error (restore (make-token-sequence raw-seq) (1+ raw-seq-len)))))

(fiveam:test test-expect
  (fiveam:with-fixture token-sequence-test-env ()
    (fiveam:is (not (null (expect (make-token-sequence raw-seq) :test-token))))
    (fiveam:is (null (expect (make-token-sequence raw-seq) :not-a-real-token)))
    (fiveam:is (= 1 (token-value (let ((s (make-token-sequence raw-seq))) (advance s) (expect s :test-token)))))))

(fiveam:test test-expect-with-value
  (fiveam:with-fixture token-sequence-test-env ()
    (fiveam:is (not (null (expect-with-value (make-token-sequence raw-seq) :test-token 0))))
    (fiveam:is (not (null (let ((s (make-token-sequence raw-seq))) (advance s) (expect-with-value s :test-token 1)))))
    (fiveam:is (null (expect-with-value (make-token-sequence raw-seq) :not-a-real-token 0)))
    (fiveam:is (null (expect-with-value (make-token-sequence raw-seq) :test-token "burger")))))