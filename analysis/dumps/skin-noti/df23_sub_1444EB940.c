// sub_1444EB940

_QWORD *__fastcall sub_1444EB940(int *a1, _QWORD *a2, __int64 a3)
{
  __int64 v5; // rax
  __int64 v6; // r8
  __int64 v7; // rsi
  _QWORD *v8; // rdx
  _QWORD *v9; // rax
  __int64 v10; // rax
  _QWORD *v11; // rdx
  unsigned __int64 v12; // r8
  __int64 v13; // r8

  *a2 = 0;
  a2[2] = 0;
  a2[3] = 7;
  *(_WORD *)a2 = 0;
  LOBYTE(a3) = 1;
  v5 = sub_140283D60(qword_14E683BF8, (unsigned int)*a1, a3);
  v7 = v5;
  if ( v5 )
  {
    v8 = (_QWORD *)(v5 + 192);
    if ( a2 != (_QWORD *)(v5 + 192) )
    {
      if ( *(_QWORD *)(v5 + 216) >= 8u )
        v8 = (_QWORD *)*v8;
      sub_14014C8D0(a2, v8);
    }
    if ( !a2[2] )
    {
      v9 = (_QWORD *)sub_147C1C550(v7, 0);
      if ( a2 != v9 )
      {
        if ( v9[3] >= 8u )
          v9 = (_QWORD *)*v9;
        goto LABEL_18;
      }
    }
  }
  else
  {
    LOBYTE(v6) = 1;
    v10 = sub_140283D60(qword_14E683B38, (unsigned int)*a1, v6);
    if ( v10 )
    {
      v11 = (_QWORD *)(v10 + 496);
      if ( a2 != (_QWORD *)(v10 + 496) )
      {
        if ( *(_QWORD *)(v10 + 520) >= 8u )
          v11 = (_QWORD *)*v11;
        goto LABEL_19;
      }
    }
    else if ( *a1 > 0 )
    {
      v12 = *a1;
      if ( v12 < (qword_14E6A7C70 - qword_14E6A7C68) / 96 )
      {
        v9 = (_QWORD *)sub_14014F430(96 * v12 + qword_14E6A7C68 + 56);
        v13 = -1;
        do
          ++v13;
        while ( *((_WORD *)v9 + v13) );
LABEL_18:
        v11 = v9;
LABEL_19:
        sub_14014C8D0(a2, v11);
      }
    }
  }
  return a2;
}

