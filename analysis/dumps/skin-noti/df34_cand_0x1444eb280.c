// cand_0x1444eb280

_QWORD *__fastcall sub_1444EB280(__int64 a1, _QWORD *a2, __int64 a3)
{
  __int64 v5; // rax
  __int64 v6; // rbx
  __int64 *v7; // r8
  __int64 *v8; // rax
  __int64 *v9; // rdx
  __int64 v10; // rcx
  _QWORD *v11; // rdx
  __int64 v12; // r8
  __int64 v13; // r8
  __int64 v14; // rax
  _QWORD *v15; // rax
  unsigned __int64 v16; // rdx
  __int64 v17; // rcx
  _QWORD *v18; // rdx
  __int64 v19; // rbx
  __int64 v20; // rax
  volatile signed __int32 *v21; // rbx
  __int64 v22; // rbx
  _QWORD *v23; // rax
  int v24; // eax
  _QWORD *v25; // rax
  __int64 v26; // rcx
  signed int v28; // [rsp+20h] [rbp-59h] BYREF
  int v29; // [rsp+28h] [rbp-51h]
  __int64 v30; // [rsp+30h] [rbp-49h]
  _QWORD *v31; // [rsp+38h] [rbp-41h]
  _BYTE v32[8]; // [rsp+40h] [rbp-39h] BYREF
  volatile signed __int32 *v33; // [rsp+48h] [rbp-31h]
  _QWORD v34[2]; // [rsp+50h] [rbp-29h] BYREF
  __int64 v35; // [rsp+60h] [rbp-19h]
  unsigned __int64 v36; // [rsp+68h] [rbp-11h]
  _OWORD v37[2]; // [rsp+70h] [rbp-9h] BYREF

  v30 = -2;
  v31 = a2;
  v28 = a3;
  v29 = 0;
  LOBYTE(a3) = 1;
  v5 = sub_140283D60(qword_14E683BF8, (unsigned int)v28, a3);
  v6 = v5;
  if ( !v5 || *(_DWORD *)(v5 + 8) != 1 )
  {
    *a2 = 0;
    a2[1] = 0;
LABEL_45:
    v29 = 1;
    return a2;
  }
  v7 = *(__int64 **)(a1 + 1080);
  v8 = (__int64 *)v7[1];
  v9 = v7;
  while ( !*((_BYTE *)v8 + 25) )
  {
    if ( *((_DWORD *)v8 + 8) >= v28 )
    {
      v9 = v8;
      v8 = (__int64 *)*v8;
    }
    else
    {
      v8 = (__int64 *)v8[2];
    }
  }
  if ( !*((_BYTE *)v9 + 25) && v28 >= *((_DWORD *)v9 + 8) && v9 != v7 && v9[5] )
  {
    *a2 = 0;
    a2[1] = 0;
    v10 = v9[6];
    if ( v10 )
      _InterlockedIncrement((volatile signed __int32 *)(v10 + 8));
    *a2 = v9[5];
    a2[1] = v9[6];
    goto LABEL_45;
  }
  v34[0] = 0;
  v35 = 0;
  v36 = 7;
  sub_14014C8D0(v34, &byte_14BAF7F08);
  v11 = *(_QWORD **)(v6 + 680);
  if ( v11 != *(_QWORD **)(v6 + 688) )
  {
    if ( v11[3] >= 8u )
      v11 = (_QWORD *)*v11;
    v12 = -1;
    do
      ++v12;
    while ( *((_WORD *)v11 + v12) );
    sub_14014C8D0(v34, v11);
    if ( v28 != 30000 && v28 != 100000 )
      goto LABEL_32;
    v14 = sub_145EFAFB0();
    v15 = sub_1444EAB20(a1, v37, v14, v28);
    if ( v34 != v15 )
    {
      if ( v15[3] >= 8u )
        v15 = (_QWORD *)*v15;
      sub_14014C8D0(v34, v15);
    }
    sub_14014C710(v37);
    if ( v35 )
    {
LABEL_32:
      v18 = v34;
      if ( v36 >= 8 )
        v18 = (_QWORD *)v34[0];
      LOBYTE(v13) = 1;
      v19 = sub_144724390(v32, v18, v13, 0);
      v20 = sub_140457BE0(a1 + 1080, &v28);
      sub_1401E5080(v20, v19);
      v21 = v33;
      if ( v33 )
      {
        if ( _InterlockedExchangeAdd(v33 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v21)(v21);
          if ( _InterlockedExchangeAdd(v21 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v21 + 8LL))(v21);
        }
      }
      v22 = *(_QWORD *)sub_140457BE0(a1 + 1080, &v28);
      v23 = (_QWORD *)sub_140457BE0(a1 + 1080, &v28);
      v24 = sub_146B33E50(*v23);
      sub_146B5E370(v22, (unsigned int)(v24 - 1), 0);
      v25 = (_QWORD *)sub_140457BE0(a1 + 1080, &v28);
      *a2 = 0;
      a2[1] = 0;
      v26 = v25[1];
      if ( v26 )
        _InterlockedIncrement((volatile signed __int32 *)(v26 + 8));
      *a2 = *v25;
      a2[1] = v25[1];
      v29 = 1;
      if ( v36 < 8 )
        goto LABEL_31;
      v16 = 2 * v36 + 2;
      v17 = v34[0];
      if ( v16 < 0x1000 )
        goto LABEL_30;
      v16 = 2 * v36 + 41;
      v17 = *(_QWORD *)(v34[0] - 8LL);
      if ( (unsigned __int64)(v34[0] - v17 - 8) <= 0x1F )
        goto LABEL_30;
      goto LABEL_47;
    }
  }
  v29 = 1;
  a2[1] = 0;
  *a2 = 0;
  if ( v36 >= 8 )
  {
    v16 = 2 * v36 + 2;
    v17 = v34[0];
    if ( v16 < 0x1000
      || (v17 = *(_QWORD *)(v34[0] - 8LL), v16 = 2 * v36 + 41, (unsigned __int64)(v34[0] - v17 - 8) <= 0x1F) )
    {
LABEL_30:
      sub_146E9F3A0(v17, v16);
      goto LABEL_31;
    }
LABEL_47:
    sub_148AAF304(v17, v16);
  }
LABEL_31:
  v35 = 0;
  v36 = 7;
  LOWORD(v34[0]) = 0;
  return a2;
}

