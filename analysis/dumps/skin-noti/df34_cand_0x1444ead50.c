// cand_0x1444ead50

__int64 __fastcall sub_1444EAD50(__int64 a1, __int64 a2, __int64 a3)
{
  unsigned int v3; // ebx
  __int64 v5; // rax
  __int64 **v6; // rdi
  __int64 *v7; // rax
  __int64 *v8; // rcx
  __int64 result; // rax
  __int64 v10; // rax
  unsigned int v11; // esi
  __int64 v12; // rdx
  __int64 v13; // rcx
  __int64 v14; // rax
  __int64 v15; // rax
  __int64 v16; // rbx
  _QWORD *v17; // rax
  _QWORD *v18; // rax
  _QWORD *v19; // rax
  _QWORD *v20; // rax
  __int64 v21; // rdx
  __int64 v22; // rcx
  __int64 v23; // rax
  unsigned int v24; // [rsp+68h] [rbp+10h] BYREF
  __int64 v25; // [rsp+78h] [rbp+20h]

  v24 = a2;
  v3 = a3;
  LOBYTE(a3) = 1;
  v5 = sub_140283D60(qword_14E683BF8, a2, a3);
  if ( !v5 || *(_DWORD *)(v5 + 8) != 2 )
    return 0;
  v6 = (__int64 **)(a1 + 1064);
  v7 = (__int64 *)(*v6)[1];
  v8 = *v6;
  while ( !*((_BYTE *)v7 + 25) )
  {
    if ( *((_DWORD *)v7 + 8) >= (signed int)v24 )
    {
      v8 = v7;
      v7 = (__int64 *)*v7;
    }
    else
    {
      v7 = (__int64 *)v7[2];
    }
  }
  if ( *((_BYTE *)v8 + 25) || (signed int)v24 < *((_DWORD *)v8 + 8) || v8 == *v6 || (result = v8[5]) == 0 )
  {
    if ( v3 <= 5 )
    {
      v10 = sub_143C61150(v8, v24);
      v11 = sub_1447EA140(v10);
      v14 = sub_143C61150(v13, v12);
      sub_142581F20(v14, v24);
      v15 = sub_146E8BA20(400);
      v25 = v15;
      if ( v15 )
        v16 = sub_1447E3ED0(v15);
      else
        v16 = 0;
      *(_QWORD *)sub_140738030(v6, &v24) = v16;
      v17 = (_QWORD *)sub_140738030(v6, &v24);
      sub_1447EB510(*v17, 0, 0, 0, 1234000, 0, 0, -1);
      v18 = (_QWORD *)sub_140738030(v6, &v24);
      sub_1447EF2D0(*v18, 0);
      v19 = (_QWORD *)sub_140738030(v6, &v24);
      sub_1447EEDB0(*v19);
      v20 = (_QWORD *)sub_140738030(v6, &v24);
      sub_1447EE700(*v20);
      v23 = sub_143C61150(v22, v21);
      sub_142581F20(v23, v11);
      return *(_QWORD *)sub_140738030(v6, &v24);
    }
    return 0;
  }
  return result;
}

