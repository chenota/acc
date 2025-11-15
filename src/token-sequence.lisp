(in-package :acc)

(defclass token-sequence ()
    ((location :accessor location :initform 0 :type (integer 0 *))
     (token-list :accessor token-list :initarg :token-list)
     (end-row :accessor end-row :initform 0 :type (integer 0 *))
     (end-col :accessor end-col :initform 0 :type (integer 0 *))
     (length :accessor len :type (integer 0 *)))
  (:documentation "Token sequence manager."))

(defmethod initialize-instance :after ((ts token-sequence) &key token-list &allow-other-keys)
  "Initialization logic for a token-sequence."
  (assert (typep token-list 'sequence))
  (assert (every #'token-p token-list))
  (setf (token-list ts) (coerce token-list 'vector))
  (setf (len ts) (length (token-list ts)))
  (when (> (len ts) 0)
        (let ((last-token (aref
                              (token-list ts)
                            (1- (len ts)))))
          (setf (end-row ts) (+ (token-row last-token) (token-len last-token)))
          (setf (end-col ts) (token-col last-token)))))

(defmethod make-token-sequence (token-list)
  "Create a token sequence"
  (make-instance 'token-sequence :token-list token-list))

(defmethod peek ((ts token-sequence))
  "Return the token at the curren position. Returns an ENDMARKER if at the end of the sequence."
  (if (= (location ts) (len ts))
      (make-token :kind :ENDMARKER :row (end-row ts) :col (end-col ts) :len 0)
      (aref (token-list ts) (location ts))))

(defmethod advance ((ts token-sequence))
  "Advance the current position. Returns an ENDMARKER if at the end of the sequence."
  (let ((current-token (peek ts)))
    (unless (eq (token-kind current-token) :ENDMARKER) (incf (location ts)))
    current-token))

(defmethod capture ((ts token-sequence))
  "Capture the current position."
  (location ts))

(defmethod restore ((ts token-sequence) pos)
  "Restore a previously captured position."
  (assert (<= pos (len ts)))
  (setf (location ts) pos))

(defmethod expect ((ts token-sequence) kind)
  "If a token of KIND is at the current position, advance, otherwise return NIL."
  (if (eq (token-kind (peek ts)) kind)
      (advance ts)))